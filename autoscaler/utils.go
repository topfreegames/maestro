// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package autoscaler

func sum(xs []float32) float32 {
	total := float32(0)
	for _, v := range xs {
		total += v
	}
	return total
}
