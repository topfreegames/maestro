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

package gen

// Protobuf
//go:generate go run github.com/bufbuild/buf/cmd/buf generate --template ../proto/api/buf.gen.yaml --output ../ --path ../proto/api/ ../

// Wire
//go:generate go run github.com/google/wire/cmd/wire ./../...

// Primary "input" ports Mocks
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/services/interfaces/events_service.go -destination=../internal/core/services/interfaces/mock/events_service/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/operation_ports.go -destination=../internal/core/ports/mock/operation_ports_mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/room_ports.go -destination=../internal/core/ports/mock/rooms_ports_mock.go -package=mock

// Secondary "output" ports Mocks
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/port_allocator.go -destination=../internal/adapters/port_allocator/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/forwarder/events_forwarder.go -destination=../internal/adapters/forwarder/events_forwarder/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/port_allocator.go -destination=../internal/adapters/port_allocator/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/runtime.go -destination=../internal/adapters/runtime/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/instance_storage.go -destination=../internal/adapters/instance_storage/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/scheduler_storage.go -destination=../internal/adapters/scheduler_storage/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/scheduler_cache.go -destination=../internal/adapters/scheduler_cache/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/ports/forwarder/forwarder_client.go -destination=../internal/adapters/forwarder/forwarder_client/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/config/config.go -destination=../internal/config/mock/mock.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/operations/definition.go -destination=../internal/core/operations/mock/definition.go -package=mock
//go:generate go run github.com/golang/mock/mockgen -source=../internal/core/operations/executor.go -destination=../internal/core/operations/mock/executor.go -package=mock

// License
//go:generate go run github.com/google/addlicense -v -skip yml -skip yaml -skip proto -f ../LICENSE ../
