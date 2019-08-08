// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package socket

import (
	"net"
	"os"
	"sync"
	"syscall"
)

// TODO: what is an optimal batch size?
const MmsgBatchSize int = 128

var mmsghdrsPool = &sync.Pool{
	New: func() interface{} {
		hs := make(mmsghdrs, MmsgBatchSize)
		return hs
	},
}

func (c *Conn) recvMsgs(ms []Message, flags int) (int, error) {
	hs := mmsghdrsPool.Get().(mmsghdrs)
	defer mmsghdrsPool.Put(hs[:MmsgBatchSize])
	hs = hs[:len(ms)]

	var parseFn func([]byte, string) (net.Addr, error)
	if c.network != "tcp" {
		// XXX disable parsing to reduce allocations
		//parseFn = parseInetAddr
	}
	if err := hs.pack(ms, parseFn, nil); err != nil {
		return 0, err
	}
	var operr error
	var n int
	fn := func(s uintptr) bool {
		n, operr = recvmmsg(s, hs, flags)
		if operr == syscall.EAGAIN {
			return false
		}
		return true
	}
	if err := c.c.Read(fn); err != nil {
		return n, err
	}
	if operr != nil {
		return n, os.NewSyscallError("recvmmsg", operr)
	}
	if err := hs[:n].unpack(ms[:n], parseFn, c.network); err != nil {
		return n, err
	}
	return n, nil
}

func (c *Conn) sendMsgs(ms []Message, flags int) (int, error) {
	hs := mmsghdrsPool.Get().(mmsghdrs)
	defer mmsghdrsPool.Put(hs[:MmsgBatchSize])
	hs = hs[:len(ms)]

	var marshalFn func(net.Addr) []byte
	if c.network != "tcp" {
		//marshalFn = marshalInetAddr
	}
	if err := hs.pack(ms, nil, marshalFn); err != nil {
		return 0, err
	}
	var operr error
	var n int
	fn := func(s uintptr) bool {
		n, operr = sendmmsg(s, hs, flags)
		if operr == syscall.EAGAIN {
			return false
		}
		return true
	}
	if err := c.c.Write(fn); err != nil {
		return n, err
	}
	if operr != nil {
		return n, os.NewSyscallError("sendmmsg", operr)
	}
	if err := hs[:n].unpack(ms[:n], nil, ""); err != nil {
		return n, err
	}
	return n, nil
}
