//+build go1.9

package jsoniter

import (
	"sync"
)

type frozenConfig struct {
	configBeforeFrozen            Config
	sortMapKeys                   bool
	indentionStep                 int
	objectFieldMustBeSimpleString bool
	onlyTaggedField               bool
	disallowUnknownFields         bool
	decoderCache                  sync.Map
	encoderCache                  sync.Map
	extensions                    []Extension
	streamPool                    *sync.Pool
	iteratorPool                  *sync.Pool
}

func (cfg *frozenConfig) initCache() {
	cfg.decoderCache = sync.Map{}
	cfg.encoderCache = sync.Map{}
}

func (cfg *frozenConfig) addDecoderToCache(cacheKey uintptr, decoder ValDecoder) {
	cfg.decoderCache.Store(cacheKey, decoder)
}

func (cfg *frozenConfig) addEncoderToCache(cacheKey uintptr, encoder ValEncoder) {
	cfg.encoderCache.Store(cacheKey, encoder)
}

func (cfg *frozenConfig) getDecoderFromCache(cacheKey uintptr) ValDecoder {
	decoder, found := cfg.decoderCache.Load(cacheKey)
	if found {
		return decoder.(ValDecoder)
	}
	return nil
}

func (cfg *frozenConfig) getEncoderFromCache(cacheKey uintptr) ValEncoder {
	encoder, found := cfg.encoderCache.Load(cacheKey)
	if found {
		return encoder.(ValEncoder)
	}
	return nil
}

var cfgCache = &sync.Map{}

func getFrozenConfigFromCache(cfg Config) *frozenConfig {
	obj, found := cfgCache.Load(cfg)
	if found {
		return obj.(*frozenConfig)
	}
	return nil
}

func addFrozenConfigToCache(cfg Config, frozenConfig *frozenConfig) {
	cfgCache.Store(cfg, frozenConfig)
}
