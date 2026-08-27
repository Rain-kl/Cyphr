package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

const maxHandlerParams = 2

var ctxInterfaceType = reflect.TypeFor[context.Context]()
var errInterfaceType = reflect.TypeFor[error]()

type eventListener struct {
	id         uint64
	fnVal      reflect.Value
	numIn      int
	hasCtx     bool
	hasPayload bool
	argType    reflect.Type
	returnsErr bool
}

// EventBus is a thread-safe, strongly-typed in-process domain event bus.
type EventBus struct {
	mu       sync.RWMutex
	nextID   atomic.Uint64
	handlers map[string][]eventListener
}

// NewEventBus creates a new EventBus instance.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]eventListener),
	}
}

// On registers an event handler for the given topic.
//
// Supported handler signatures:
//   - func(ctx context.Context, event T) error
//   - func(ctx context.Context, event T)
//   - func(event T) error
//   - func(event T)
//   - func(ctx context.Context) error
//   - func(ctx context.Context)
//   - func() error
//   - func()
//
// Returns a Disposer function that unregisters the handler when called.
func (b *EventBus) On(topic string, handler any) Disposer {
	if handler == nil {
		panic("core/events: handler cannot be nil")
	}

	fnVal := reflect.ValueOf(handler)
	fnType := fnVal.Type()

	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("core/events: expected func, got %s", fnType.Kind()))
	}

	numIn := fnType.NumIn()
	if numIn > maxHandlerParams {
		panic(fmt.Sprintf("core/events: handler has %d parameters, maximum 2 supported (ctx, event)", numIn))
	}

	numOut := fnType.NumOut()
	if numOut > 1 {
		panic(fmt.Sprintf("core/events: handler has %d return values, maximum 1 supported (error)", numOut))
	}

	returnsErr := false
	if numOut == 1 {
		outType := fnType.Out(0)
		if !outType.Implements(errInterfaceType) {
			panic(fmt.Sprintf("core/events: handler return type must be error, got %v", outType))
		}
		returnsErr = true
	}

	listener := eventListener{
		id:         b.nextID.Add(1),
		fnVal:      fnVal,
		numIn:      numIn,
		returnsErr: returnsErr,
	}

	switch numIn {
	case 0:
		// func() or func() error
	case 1:
		in0 := fnType.In(0)
		if in0.Implements(ctxInterfaceType) {
			listener.hasCtx = true
		} else {
			listener.hasPayload = true
			listener.argType = in0
		}
	case 2:
		in0 := fnType.In(0)
		if !in0.Implements(ctxInterfaceType) {
			panic(fmt.Sprintf("core/events: first parameter must implement context.Context, got %v", in0))
		}
		listener.hasCtx = true
		listener.hasPayload = true
		listener.argType = fnType.In(1)
	}

	b.mu.Lock()
	b.handlers[topic] = append(b.handlers[topic], listener)
	b.mu.Unlock()

	listenerID := listener.id
	var disposed atomic.Bool

	return func() error {
		if disposed.Swap(true) {
			return nil
		}

		b.mu.Lock()
		defer b.mu.Unlock()

		list := b.handlers[topic]
		for i, l := range list {
			if l.id == listenerID {
				b.handlers[topic] = append(list[:i], list[i+1:]...)
				break
			}
		}

		if len(b.handlers[topic]) == 0 {
			delete(b.handlers, topic)
		}

		return nil
	}
}

// Subscribe registers a strongly-typed generic event listener on the given EventBus.
func Subscribe[T any](bus *EventBus, topic string, handler func(ctx context.Context, event T) error) Disposer {
	if bus == nil {
		panic("core/events: nil EventBus provided to Subscribe")
	}
	return bus.On(topic, handler)
}

// Emit publishes an event to all subscribers of the specified topic.
// Handlers are executed synchronously. If any handler panics or returns an error,
// the error is collected and returned via errors.Join.
//
//nolint:contextcheck
func (b *EventBus) Emit(ctx context.Context, topic string, payload any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	b.mu.RLock()
	rawListeners := b.handlers[topic]
	if len(rawListeners) == 0 {
		b.mu.RUnlock()
		return nil
	}

	listeners := make([]eventListener, len(rawListeners))
	copy(listeners, rawListeners)
	b.mu.RUnlock()

	var payloadVal reflect.Value
	if payload != nil {
		payloadVal = reflect.ValueOf(payload)
	}

	var errs []error
	for _, l := range listeners {
		args := b.buildArgs(ctx, l, payloadVal)

		err := func() (resErr error) {
			defer func() {
				if r := recover(); r != nil {
					resErr = fmt.Errorf("core/events: panic in handler for topic %q: %v", topic, r)
				}
			}()

			results := l.fnVal.Call(args)
			if l.returnsErr && len(results) > 0 && !results[0].IsNil() {
				resErr = results[0].Interface().(error)
			}
			return resErr
		}()

		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (b *EventBus) buildArgs(ctx context.Context, l eventListener, payloadVal reflect.Value) []reflect.Value {
	if l.numIn == 0 {
		return nil
	}

	args := make([]reflect.Value, 0, l.numIn)
	if l.hasCtx {
		args = append(args, reflect.ValueOf(ctx))
	}

	if l.hasPayload {
		arg := b.convertPayload(payloadVal, l.argType)
		args = append(args, arg)
	}

	return args
}

func (b *EventBus) convertPayload(payloadVal reflect.Value, targetType reflect.Type) reflect.Value {
	if !payloadVal.IsValid() {
		return reflect.Zero(targetType)
	}

	valType := payloadVal.Type()

	// 1. Direct assignable
	if valType.AssignableTo(targetType) {
		return payloadVal
	}

	// 2. Direct convertible
	if valType.ConvertibleTo(targetType) {
		return payloadVal.Convert(targetType)
	}

	// 3. Payload is pointer *T, target expects T
	if valType.Kind() == reflect.Pointer && valType.Elem().AssignableTo(targetType) {
		if !payloadVal.IsNil() {
			return payloadVal.Elem()
		}
		return reflect.Zero(targetType)
	}

	// 4. Payload is value T, target expects *T
	if targetType.Kind() == reflect.Pointer && valType.AssignableTo(targetType.Elem()) {
		ptr := reflect.New(valType)
		ptr.Elem().Set(payloadVal)
		return ptr
	}

	// Fallback to zero value of targetType
	return reflect.Zero(targetType)
}

// Listeners returns the number of active listeners for a topic.
func (b *EventBus) Listeners(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[topic])
}

// Topics returns all topics that have registered listeners.
func (b *EventBus) Topics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topics := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		topics = append(topics, t)
	}
	return topics
}
