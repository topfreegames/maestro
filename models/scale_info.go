// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

//ScaleInfo holds information about last time scheduler was verified if it needed to be scaled
// and how many time it was above or below threshold
//TODO: if time is changed on the yaml, the watcher (or the worker) needs to restart =/
type ScaleInfo struct {
	pointsAboveUsage int
	points           []float32
	pointer          int
	length           int
}

// NewScaleInfo returns a new ScaleInfo
func NewScaleInfo(cap int) *ScaleInfo {
	return &ScaleInfo{
		points: make([]float32, cap),
	}
}

// AddPoint inserts a new point on a circular list and updates pointsAboveUsage
func (s *ScaleInfo) AddPoint(point, total int, usage float32) {
	if s.length >= len(s.points) {
		s.length = len(s.points) - 1
		if s.points[s.pointer] >= usage {
			s.pointsAboveUsage = s.pointsAboveUsage - 1
		}
	}

	currentUsage := float32(point) / float32(total)
	s.points[s.pointer] = currentUsage
	s.length = s.length + 1
	s.pointer = (s.pointer + 1) % cap(s.points)
	if currentUsage >= usage {
		s.pointsAboveUsage = s.pointsAboveUsage + 1
	}
}

// IsAboveThreshold returns true if the percentage of points above usage is greater than threshold
func (s *ScaleInfo) IsAboveThreshold(threshold int) bool {
	return 100*s.pointsAboveUsage >= threshold*s.length
}

// GetPoints returns the array of points, where each point is the usage at that time
func (s *ScaleInfo) GetPoints() []float32 {
	return s.points
}

// GetPointsAboveUsage returns the total number of points that were above usage
func (s *ScaleInfo) GetPointsAboveUsage() int {
	return s.pointsAboveUsage
}
