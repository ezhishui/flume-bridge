package consumer

import (
	"flume-log-sdk/config"
	"flume-log-sdk/consumer/pool"
	"log"
)

type FlumeWatcher struct {
	sinkmanager *SinkManager
	business    string
}

func newFlumeWatcher(business string, sinkmanager *SinkManager) *config.Watcher {
	flumeWatcher := &FlumeWatcher{business: business, sinkmanager: sinkmanager}
	return config.NewWatcher(business, flumeWatcher)
}

func (self *FlumeWatcher) BusinessWatcher(business string, eventType config.ZkEvent) {
	//当前节点有发生变更,只关注删除该节点就行
	if eventType == config.Deleted {
		self.sinkmanager.mutex.Lock()
		defer self.sinkmanager.mutex.Unlock()
		val, ok := self.sinkmanager.sinkServers[business]
		if ok {
			//关闭这个业务消费
			val.stop()
			delete(self.sinkmanager.sinkServers, business)
			for _, fpool := range val.flumeClientPool {
				if fpool.BusinessLink.Len() == 0 {
					//如果已经没有使用的业务了直接关掉该pool
					fpool.FlumePool.Destroy()
					hp := fpool.FlumePool.GetHostPort()
					delete(self.sinkmanager.hp2flumeClientPool, fpool.FlumePool.GetHostPort())
					log.Printf("remove flume agent :[%s]", hp)
				}
			}

			log.Printf("business:[%s] deleted\n", business)
		} else {
			log.Printf("business:[%s] not exist !\n", business)
		}
	}
}

func (self *FlumeWatcher) ChildWatcher(business string, childNode []config.HostPort) {
	//当前业务下的flume节点发生了变更会全量推送一次新的节点

	if len(childNode) <= 0 {
		self.BusinessWatcher(business, config.Deleted)
		return
	}

	self.sinkmanager.mutex.Lock()
	defer self.sinkmanager.mutex.Unlock()
	val, ok := self.sinkmanager.sinkServers[business]
	if ok {
		//已经存在那么就检查节点变更
		for _, hp := range childNode {
			//先创建该业务节点：
			fpool, ok := self.sinkmanager.hp2flumeClientPool[hp]
			//如果存在Pool直接使用
			if ok {
				contain := false
				//检查该业务已有是否已经该flumepool
				for e := fpool.BusinessLink.Back(); nil != e; e = e.Prev() {
					if e.Value.(string) == business {
						contain = true
						break
					}
				}

				//如果不包含则创建该池子并加入该业务对应的flumeclientpoollink中
				if !contain {
					val.flumeClientPool = append(val.flumeClientPool, fpool)
					log.Printf("business:[%s] add flume :[\n", business, fpool)
				}
				//如果已经包含了，则啥事都不干

			} else {
				//如果不存在该flumepool，直接创建并且添加到该pool种
				err, poollink := pool.NewFlumePoolLink(hp)
				if nil != err {
					self.sinkmanager.hp2flumeClientPool[hp] = poollink
					val.flumeClientPool = append(val.flumeClientPool, poollink)
					poollink.BusinessLink.PushFront(business)
				}
			}
		}

	} else {
		self.sinkmanager.initSinkServer(business, childNode)
	}
}