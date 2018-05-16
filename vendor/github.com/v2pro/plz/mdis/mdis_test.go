package mdis_test

import (
	"testing"
	"github.com/v2pro/plz/test"
	"github.com/v2pro/plz/countlog"
	"github.com/v2pro/plz/test/must"
	"github.com/v2pro/plz/mdis"
)

type myOrder struct {
	productId int
	scenario  int
	brand     int
}

var myFunc = func(order *myOrder, arg1 string) string {
	return "orig " + arg1
}

func Test_mdis(t *testing.T) {
	t.Run("orig", test.Case(func(ctx *countlog.Context) {
		must.Equal("orig arg1", myFunc(&myOrder{}, "arg1"))
	}))
	t.Run("by exact much", test.Case(func(ctx *countlog.Context) {
		mdis.Register(&myFunc, &myOrder{
			productId: 1,
			scenario:  2,
			brand:     3,
		}, func(order *myOrder, arg1 string) string {
			return "dispatched " + arg1
		})
		must.Equal("orig arg1", myFunc(&myOrder{}, "arg1"))
		must.Equal("dispatched arg1", myFunc(&myOrder{
			productId: 1,
			scenario:  2,
			brand:     3,
		}, "arg1"))
	}))
	t.Run("by table", test.Case(func(ctx *countlog.Context) {
		selector := func(order *myOrder) string {
			if order.productId == 1 && order.brand > 2 {
				return "vip product"
			}
			return "normal product"
		}
		mdis.RegisterTable(&myFunc, selector,
			"vip product", func(order *myOrder, arg1 string) string {
				return "vip " + arg1
			},
			"normal product", func(order *myOrder, arg1 string) string {
				return "normal " + arg1
			})
		must.Equal("vip arg1", myFunc(&myOrder{
			productId: 1,
			scenario:  2,
			brand:     3,
		}, "arg1"))
		must.Equal("normal arg1", myFunc(&myOrder{
			productId: 1,
			scenario:  2,
			brand:     2,
		}, "arg1"))
	}))
}
