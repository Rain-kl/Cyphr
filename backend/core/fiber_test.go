// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core"
)

type MockServiceA interface {
	DoA() string
}

type mockServiceAImpl struct{}

func (m *mockServiceAImpl) DoA() string { return "doneA" }

type MockServiceB interface {
	DoB() string
}

type mockServiceBImpl struct{}

func (m *mockServiceBImpl) DoB() string { return "doneB" }

type mockProviderPlugin struct {
	name    string
	applied bool
}

func (p *mockProviderPlugin) Name() string {
	return p.name
}

func (p *mockProviderPlugin) Apply(ctx *core.Context) error {
	p.applied = true
	core.Provide[MockServiceA](ctx, &mockServiceAImpl{})
	return nil
}

type mockConsumerPlugin struct {
	name    string
	applied bool
	gotSvc  MockServiceA
}

func (p *mockConsumerPlugin) Name() string {
	return p.name
}

func (p *mockConsumerPlugin) Inject() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[MockServiceA](),
	}
}

func (p *mockConsumerPlugin) Apply(ctx *core.Context) error {
	p.applied = true
	svc, err := core.Inject[MockServiceA](ctx)
	if err != nil {
		return err
	}
	p.gotSvc = svc
	return nil
}

func TestFiber_ConfluenceAndReactiveActivation(t *testing.T) {
	app := core.NewApp()

	// Register Consumer BEFORE Provider to test confluence & out-of-order dependency resolution
	consumer := &mockConsumerPlugin{name: "consumer-plugin"}
	provider := &mockProviderPlugin{name: "provider-plugin"}

	app.Use(consumer, provider)

	err := app.Start(context.Background())
	require.NoError(t, err)

	assert.True(t, provider.applied, "provider should be applied")
	assert.True(t, consumer.applied, "consumer should be reactively applied once dependency was provided")
	assert.NotNil(t, consumer.gotSvc)
	assert.Equal(t, "doneA", consumer.gotSvc.DoA())

	fibers := app.Fibers()
	require.Equal(t, 2, len(fibers))
	for _, f := range fibers {
		assert.Equal(t, core.FiberActive, f.State())
	}

	err = app.Stop()
	assert.NoError(t, err)
}

func TestFiber_UnsatisfiedDependencyReturnsError(t *testing.T) {
	app := core.NewApp()

	// Register Consumer whose dependency is never provided
	consumer := &mockConsumerPlugin{name: "consumer-plugin"}
	app.Use(consumer)

	err := app.Start(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsatisfied dependencies")
}
