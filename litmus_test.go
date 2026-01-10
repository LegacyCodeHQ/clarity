package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvironmentIsSetup(t *testing.T) {
	actual := 2 + 2
	expected := 4

	assert.Equal(t, expected, actual)
}
