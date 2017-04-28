// Expose some internals for testing purposes
package lz4

// expose the possible block max sizes
var BlockMaxSizeItems []int

func init() {
	for s := range bsMapValue {
		BlockMaxSizeItems = append(BlockMaxSizeItems, s)
	}
}

var FrameSkipMagic = frameSkipMagic
