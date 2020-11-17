package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_convertGraphite(t *testing.T) {
	var success bool
	builder := bytes.NewBuffer(make([]byte, 1024))

	builder.Reset()
	success = convertGraphite(builder, []byte("a 1 1"))
	assert.False(t, success)

	builder.Reset()
	success = convertGraphite(builder, []byte("a.b.c 11"))
	assert.False(t, success)

	builder.Reset()
	success = convertGraphite(builder, []byte("a.b.c 1 1 1"))
	assert.False(t, success)

	builder.Reset()
	success = convertGraphite(builder, []byte("a.b.c 1 1"))
	assert.True(t, success)
	assert.Equal(t, "a;__a_g1__=b;__a_g2__=c 1 1\n", builder.String())
}
