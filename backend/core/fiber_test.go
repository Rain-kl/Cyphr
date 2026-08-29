// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"Wavelet/core"
	"Wavelet/core/extpoints"
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// gateConfig is the configuration section consumed by gatedPlugin below.
type gateConfig struct {
	Enabled bool `config:"enabled" env:"GATE_ENABLED"`
}

// gatedPlugin implements core.ConfigGatedPlugin: two instances with opposite
// expectations model the mutually exclusive cache / cache_memory driver pair.
type gatedPlugin struct {
	name    string
	enabled bool
	applied bool
}

func (g *gatedPlugin) Name() string { return g.name }

func (g *gatedPlugin) Apply(_ *core.Context) error {
	g.applied = true
	return nil
}

func (g *gatedPlugin) DeclareConfig() []extpoints.ConfigBinding {
	return []extpoints.ConfigBinding{{Prefix: "gate", Target: &gateConfig{}}}
}

func (g *gatedPlugin) ConfigEnabled(view extpoints.ConfigView) bool {
	return view.Bool("gate.enabled", false) == g.enabled
}

func TestFiberSkipMovesToSkippedStateAndDisposesScope(t *testing.T) {
	root := core.NewContext(nil)
	plugin := &gatedPlugin{name: "cache", enabled: true}
	f := core.NewFiber(root, plugin)
	require.Equal(t, core.FiberPending, f.State())

	require.NoError(t, f.Skip())

	assert.Equal(t, core.FiberSkipped, f.State())
	assert.True(t, f.Skipped())
	assert.False(t, plugin.applied, "a skipped plugin must never reach Apply")
	assert.NoError(t, f.Unload(), "unloading a skipped fiber is a no-op")
}

func TestFiberSkipIsInertForActiveFibers(t *testing.T) {
	root := core.NewContext(nil)
	f := core.NewFiber(root, &gatedPlugin{name: "cache", enabled: true})
	require.NoError(t, f.Load())

	require.NoError(t, f.Skip())

	assert.Equal(t, core.FiberActive, f.State(), "Skip only applies to pending fibers")
	assert.False(t, f.Skipped())
}
