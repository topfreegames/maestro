// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package game_room

type Container struct {
	Name            string   `validate:"min=1"`
	Image           string   `validate:"min=1"`
	ImagePullPolicy string   `validate:"regexp=^(Always|Never|IfNotPresent){1}$"`
	Command         []string `validate:"min=1"`
	Environment     []ContainerEnvironment
	Requests        ContainerResources
	Limits          ContainerResources
	Ports           []ContainerPort `validate:"min=1"`
}

type ContainerEnvironment struct {
	Name  string `validate:"min=1"`
	Value string
}

type ContainerResources struct {
	Memory string `validate:"min=1"`
	CPU    string `validate:"min=1"`
}

type ContainerPort struct {
	Name     string `validate:"min=1"`
	Protocol string `validate:"min=1"`
	Port     int    `validate:"min=1"`
	HostPort int
}
