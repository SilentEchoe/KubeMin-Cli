package queue

import "time"

func InitQueue() error {
	//从数据库中加载未完成的任务
	//TODO Mock Debug
	return nil
}

func WorkflowTaskSender() {
	for {
		time.Sleep(time.Second * 3)

		// 这里默认使用redis做分布式锁
		// 获取系统设置
		// 获取等待的任务

	}
}
