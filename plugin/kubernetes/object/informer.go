package object

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// NewIndexerInformer is a copy of the cache.NewIndexerInformer function, but allows custom process function
func NewIndexerInformer(lw cache.ListerWatcher, objType runtime.Object, h cache.ResourceEventHandler, indexers cache.Indexers, builder ProcessorBuilder) (cache.Indexer, cache.Controller) {
	clientState := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, indexers)

	cfg := &cache.Config{
		Queue:            cache.NewDeltaFIFO(cache.MetaNamespaceKeyFunc, clientState),
		ListerWatcher:    lw,
		ObjectType:       objType,
		FullResyncPeriod: defaultResyncPeriod,
		RetryOnError:     false,
		Process:          builder(clientState, h),
	}
	return clientState, cache.New(cfg)
}

// DefaultProcessor is a copy of Process function from cache.NewIndexerInformer except it does a conversion.
func DefaultProcessor(convert ToFunc) ProcessorBuilder {
	return func(clientState cache.Indexer, h cache.ResourceEventHandler) cache.ProcessFunc {
		return func(obj interface{}) error {
			for _, d := range obj.(cache.Deltas) {

				obj := convert(d.Object)

				switch d.Type {
				case cache.Sync, cache.Added, cache.Updated:
					if old, exists, err := clientState.Get(obj); err == nil && exists {
						if err := clientState.Update(obj); err != nil {
							return err
						}
						h.OnUpdate(old, obj)
					} else {
						if err := clientState.Add(obj); err != nil {
							return err
						}
						h.OnAdd(obj)
					}
				case cache.Deleted:
					if err := clientState.Delete(obj); err != nil {
						return err
					}
					h.OnDelete(obj)
				}
			}
			return nil
		}
	}
}

const defaultResyncPeriod = 0
