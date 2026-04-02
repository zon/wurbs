package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSequence_String_Empty(t *testing.T) {
	s := Sequence{}
	assert.Equal(t, "[]", s.String())
}

func TestSequence_String_SingleElement(t *testing.T) {
	s := Sequence{42}
	assert.Equal(t, "[42]", s.String())
}

func TestSequence_String_MultipleElements(t *testing.T) {
	s := Sequence{1, 2, 3}
	assert.Equal(t, "[1, 2, 3]", s.String())
}

func TestSequence_String_LargeValues(t *testing.T) {
	s := Sequence{100, 999999, 1}
	assert.Equal(t, "[100, 999999, 1]", s.String())
}
