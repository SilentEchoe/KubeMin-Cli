package container

import (
	"k8s.io/klog/v2"
	"time"
)

type Container struct {
}

func NewContainer() *Container {
	return &Container{}
}

func (c *Container) Provides(beans ...interface{}) error {

	return nil
}

func (c *Container) ProvideWithName(name string, bean interface{}) error {
	return nil
}

func (c *Container) Populate() error {
	start := time.Now()
	defer func() {
		klog.Infof("populate the bean container take time %s", time.Now().Sub(start))
	}()
	return nil
}
