package reversi

// Canonical encodes the state into stable bytes: the canonical input of
// data:state-checkpoint. Field order is fixed here forever; the eventual
// contract for wire-visible games is the generated CBOR world profile
// encoding, which this sample does not need yet.
func Canonical(s *State) []byte {
	b := make([]byte, 0, 68)
	for _, c := range s.Board {
		b = append(b, byte(c))
	}
	b = append(b, byte(s.Next>>8), byte(s.Next), s.Passes, boolByte(s.Over))
	return b
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}
