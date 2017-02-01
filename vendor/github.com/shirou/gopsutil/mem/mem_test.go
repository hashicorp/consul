package mem

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVirtual_memory(t *testing.T) {
	v, err := VirtualMemory()
	if err != nil {
		t.Errorf("error %v", err)
	}
	empty := &VirtualMemoryStat{}
	if v == empty {
		t.Errorf("error %v", v)
	}

	assert.True(t, v.Total > 0)
	assert.True(t, v.Available > 0)
	assert.True(t, v.Used > 0)

	assert.Equal(t, v.Total, v.Available+v.Used,
		"Total should be computable from available + used: %v", v)

	assert.True(t, v.Free > 0)
	assert.True(t, v.Available > v.Free,
		"Free should be a subset of Available: %v", v)

	assert.InDelta(t, v.UsedPercent,
		100*float64(v.Used)/float64(v.Total), 0.1,
		"UsedPercent should be how many percent of Total is Used: %v", v)
}

func TestSwap_memory(t *testing.T) {
	v, err := SwapMemory()
	if err != nil {
		t.Errorf("error %v", err)
	}
	empty := &SwapMemoryStat{}
	if v == empty {
		t.Errorf("error %v", v)
	}
}

func TestVirtualMemoryStat_String(t *testing.T) {
	v := VirtualMemoryStat{
		Total:       10,
		Available:   20,
		Used:        30,
		UsedPercent: 30.1,
		Free:        40,
	}
	e := `{"total":10,"available":20,"used":30,"usedPercent":30.1,"free":40,"active":0,"inactive":0,"wired":0,"buffers":0,"cached":0,"writeback":0,"dirty":0,"writebacktmp":0,"shared":0,"slab":0,"pagetables":0,"swapcached":0}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("VirtualMemoryStat string is invalid: %v", v)
	}
}

func TestSwapMemoryStat_String(t *testing.T) {
	v := SwapMemoryStat{
		Total:       10,
		Used:        30,
		Free:        40,
		UsedPercent: 30.1,
	}
	e := `{"total":10,"used":30,"free":40,"usedPercent":30.1,"sin":0,"sout":0}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("SwapMemoryStat string is invalid: %v", v)
	}
}
