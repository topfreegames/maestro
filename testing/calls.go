// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

import "github.com/golang/mock/gomock"

type Calls struct {
	CallList []*gomock.Call
}

func NewCalls() *Calls {
	return &Calls{
		CallList: []*gomock.Call{},
	}
}

func (c *Calls) Add(call *gomock.Call) {
	c.CallList = append(c.CallList, call)
}

func (c *Calls) Finish() {
	gomock.InOrder(c.CallList...)
}
