package f32

// ScalUnitary is
//  for i := range x {
//  	x[i] *= alpha
//  }
func ScalUnitary(alpha float32, x []float32) {
	for i := range x {
		x[i] *= alpha
	}
}

// ScalUnitaryTo is
//  for i, v := range x {
//  	dst[i] = alpha * v
//  }
func ScalUnitaryTo(dst []float32, alpha float32, x []float32) {
	for i, v := range x {
		dst[i] = alpha * v
	}
}

// Add is
//  for i, v := range s {
//  	dst[i] += v
//  }
func Add(dst, s []float32) {
	for i, v := range s {
		dst[i] += v
	}
}

// AddConst is
//  for i := range x {
//  	x[i] += alpha
//  }
func AddConst(alpha float32, x []float32) {
	for i := range x {
		x[i] += alpha
	}
}

// Div is
//  for i, v := range s {
//  	dst[i] /= v
//  }
func Div(dst, s []float32) {
	for i, v := range s {
		dst[i] /= v
	}
}

// DivTo is
//  for i, v := range s {
//  	dst[i] = v / t[i]
//  }
//  return dst
func DivTo(dst, s, t []float32) []float32 {
	for i, v := range s {
		dst[i] = v / t[i]
	}
	return dst
}

// Sum is
//  var sum float32
//  for i := range x {
//      sum += x[i]
//  }
func Sum(x []float32) float32 {
	var sum float32
	for _, v := range x {
		sum += v
	}
	return sum
}
