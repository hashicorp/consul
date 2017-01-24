//go:generate sh -c "go run watch-gen/main.go >watch_few.go"
package memdb

import(
	"time"
)

// aFew gives how many watchers this function is wired to support. You must
// always pass a full slice of this length, but unused channels can be nil.
const aFew = 32

// watchFew is used if there are only a few watchers as a performance
// optimization.
func watchFew(ch []<-chan struct{}, timeoutCh <-chan time.Time) bool {
	select {

	case <-ch[0]:
		return false

	case <-ch[1]:
		return false

	case <-ch[2]:
		return false

	case <-ch[3]:
		return false

	case <-ch[4]:
		return false

	case <-ch[5]:
		return false

	case <-ch[6]:
		return false

	case <-ch[7]:
		return false

	case <-ch[8]:
		return false

	case <-ch[9]:
		return false

	case <-ch[10]:
		return false

	case <-ch[11]:
		return false

	case <-ch[12]:
		return false

	case <-ch[13]:
		return false

	case <-ch[14]:
		return false

	case <-ch[15]:
		return false

	case <-ch[16]:
		return false

	case <-ch[17]:
		return false

	case <-ch[18]:
		return false

	case <-ch[19]:
		return false

	case <-ch[20]:
		return false

	case <-ch[21]:
		return false

	case <-ch[22]:
		return false

	case <-ch[23]:
		return false

	case <-ch[24]:
		return false

	case <-ch[25]:
		return false

	case <-ch[26]:
		return false

	case <-ch[27]:
		return false

	case <-ch[28]:
		return false

	case <-ch[29]:
		return false

	case <-ch[30]:
		return false

	case <-ch[31]:
		return false

	case <-timeoutCh:
		return true
	}
}
