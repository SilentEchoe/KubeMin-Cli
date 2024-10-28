package workqueue

import (
	"k8s.io/utils/clock"
	"sync"
)

type Interface interface {
	Add(item interface{})
	Len() int
	Get() (item interface{}, shutdown bool) // 获取一个元素 第二个返回值和 Channel 类似，标记队列是否关闭
	Done(item interface{})                  // 标记一个元素已经处理完毕
	ShutDown()                              // 关闭
	ShuttingDown() bool                     // 标记当前Channel 是否正在关闭
}

type Type struct {
	queue      []t
	dirty      set
	processing set
	cond       *sync.Cond
	clock      clock.WithTicker
}

type empty struct{}
type t interface{}
type set map[t]empty
