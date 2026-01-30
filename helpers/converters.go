package helpers

func Int64ToFloat32(n *int64) *float32 {
	if n == nil {
		return nil
	}
	f := float32(*n)
	return &f
}

func BooleanToInt(b *bool) *int {
	if b == nil {
		return nil
	}
	if *b {
		i := 1
		return &i
	} else {
		i := 0
		return &i
	}
}
