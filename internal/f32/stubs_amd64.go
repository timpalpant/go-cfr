// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !noasm,!appengine,!safe

package f32

// AxpyUnitary is
//  for i, v := range x {
//  	y[i] += alpha * v
//  }
func AxpyUnitary(alpha float32, x, y []float32)

// DotUnitary is
//  for i, v := range x {
//  	sum += y[i] * v
//  }
//  return sum
func DotUnitary(x, y []float32) (sum float32)

// TODO: Migrate from gonum/f64:
// - ScalUnitary
// - ScalUnitaryTo
// - Sum
// - Div
// - DivTo
// - AddConst
