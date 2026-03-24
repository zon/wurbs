package message

import "strings"

type Sequence []uint

func (s Sequence) String() string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, id := range s {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(uintToString(id))
	}
	sb.WriteString("]")
	return sb.String()
}

func uintToString(n uint) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
