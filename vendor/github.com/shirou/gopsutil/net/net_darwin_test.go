package net

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

const (
	netstatTruncated = `Name  Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll Drop
lo0   16384 <Link#1>                         31241     0    3769823    31241     0    3769823     0   0
lo0   16384 ::1/128     ::1                  31241     -    3769823    31241     -    3769823     -   -
lo0   16384 127           127.0.0.1          31241     -    3769823    31241     -    3769823     -   -
lo0   16384 fe80::1%lo0 fe80:1::1            31241     -    3769823    31241     -    3769823     -   -
gif0* 1280  <Link#2>                             0     0          0        0     0          0     0   0
stf0* 1280  <Link#3>                             0     0          0        0     0          0     0   0
utun8 1500  <Link#88>                          286     0      27175        0     0          0     0   0
utun8 1500  <Link#90>                          286     0      29554        0     0          0     0   0
utun8 1500  <Link#92>                          286     0      29244        0     0          0     0   0
utun8 1500  <Link#93>                          286     0      28267        0     0          0     0   0
utun8 1500  <Link#95>                          286     0      28593        0     0          0     0   0`
	netstatNotTruncated  = `Name  Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll Drop
lo0   16384 <Link#1>                      27190978     0 12824763793 27190978     0 12824763793     0   0
lo0   16384 ::1/128     ::1               27190978     - 12824763793 27190978     - 12824763793     -   -
lo0   16384 127           127.0.0.1       27190978     - 12824763793 27190978     - 12824763793     -   -
lo0   16384 fe80::1%lo0 fe80:1::1         27190978     - 12824763793 27190978     - 12824763793     -   -
gif0* 1280  <Link#2>                             0     0          0        0     0          0     0   0
stf0* 1280  <Link#3>                             0     0          0        0     0          0     0   0
en0   1500  <Link#4>    a8:66:7f:dd:ee:ff  5708989     0 7295722068  3494252     0  379533492     0 230
en0   1500  fe80::aa66: fe80:4::aa66:7fff  5708989     - 7295722068  3494252     -  379533492     -   -`
)

func TestparseNetstatLineHeader(t *testing.T) {
	stat, linkIkd, err := parseNetstatLine(`Name  Mtu   Network       Address            Ipkts Ierrs     Ibytes    Opkts Oerrs     Obytes  Coll Drop`)
	assert.Nil(t, linkIkd)
	assert.Nil(t, stat)
	assert.Error(t, err)
	assert.Equal(t, errNetstatHeader, err)
}

func assertLoopbackStat(t *testing.T, err error, stat *IOCountersStat) {
	assert.NoError(t, err)
	assert.Equal(t, 869107, stat.PacketsRecv)
	assert.Equal(t, 0, stat.Errin)
	assert.Equal(t, 169411755, stat.BytesRecv)
	assert.Equal(t,869108,  stat.PacketsSent)
	assert.Equal(t, 1, stat.Errout)
	assert.Equal(t, 169411756, stat.BytesSent)
}

func TestparseNetstatLineLink(t *testing.T) {
	stat, linkId, err := parseNetstatLine(
		`lo0   16384 <Link#1>                        869107     0  169411755   869108     1  169411756     0   0`,
	)
	assertLoopbackStat(t, err, stat)
	assert.NotNil(t, linkId)
	assert.Equal(t, uint(1), *linkId)
}

func TestparseNetstatLineIPv6(t *testing.T) {
	stat, linkId, err := parseNetstatLine(
		`lo0   16384 ::1/128     ::1                 869107     -  169411755   869108     1  169411756     -   -`,
	)
	assertLoopbackStat(t, err, stat)
	assert.Nil(t, linkId)
}

func TestparseNetstatLineIPv4(t *testing.T) {
	stat, linkId, err := parseNetstatLine(
		`lo0   16384 127           127.0.0.1         869107     -  169411755   869108     1  169411756     -   -`,
	)
	assertLoopbackStat(t, err, stat)
	assert.Nil(t, linkId)
}

func TestParseNetstatOutput(t *testing.T) {
	nsInterfaces, err := parseNetstatOutput(netstatNotTruncated)
	assert.NoError(t, err)
	assert.Len(t, nsInterfaces, 8)
	for index := range nsInterfaces {
		assert.NotNil(t, nsInterfaces[index].stat, "Index %d", index)
	}

	assert.NotNil(t, nsInterfaces[0].linkId)
	assert.Equal(t, uint(1), *nsInterfaces[0].linkId)

	assert.Nil(t, nsInterfaces[1].linkId)
	assert.Nil(t, nsInterfaces[2].linkId)
	assert.Nil(t, nsInterfaces[3].linkId)

	assert.NotNil(t, nsInterfaces[4].linkId)
	assert.Equal(t, uint(2), *nsInterfaces[4].linkId)

	assert.NotNil(t, nsInterfaces[5].linkId)
	assert.Equal(t, uint(3), *nsInterfaces[5].linkId)

	assert.NotNil(t, nsInterfaces[6].linkId)
	assert.Equal(t,  uint(4), *nsInterfaces[6].linkId)

	assert.Nil(t, nsInterfaces[7].linkId)

	mapUsage := newMapInterfaceNameUsage(nsInterfaces)
	assert.False(t, mapUsage.isTruncated())
	assert.Len(t, mapUsage.notTruncated(), 4)
}

func TestParseNetstatTruncated(t *testing.T) {
	nsInterfaces, err := parseNetstatOutput(netstatTruncated)
	assert.NoError(t, err)
	assert.Len(t, nsInterfaces, 11)
	for index := range nsInterfaces {
		assert.NotNil(t, nsInterfaces[index].stat, "Index %d", index)
	}

	const truncatedIface = "utun8"

	assert.NotNil(t, nsInterfaces[6].linkId)
	assert.Equal(t,  uint(88), *nsInterfaces[6].linkId)
	assert.Equal(t, truncatedIface, nsInterfaces[6].stat.Name)

	assert.NotNil(t, nsInterfaces[7].linkId)
	assert.Equal(t,uint(90), *nsInterfaces[7].linkId)
	assert.Equal(t, truncatedIface, nsInterfaces[7].stat.Name)

	assert.NotNil(t, nsInterfaces[8].linkId)
	assert.Equal(t, uint(92), *nsInterfaces[8].linkId )
	assert.Equal(t, truncatedIface, nsInterfaces[8].stat.Name)

	assert.NotNil(t, nsInterfaces[9].linkId)
	assert.Equal(t, uint(93), *nsInterfaces[9].linkId )
	assert.Equal(t, truncatedIface, nsInterfaces[9].stat.Name)

	assert.NotNil(t, nsInterfaces[10].linkId)
	assert.Equal(t, uint(95), *nsInterfaces[10].linkId )
	assert.Equal(t, truncatedIface, nsInterfaces[10].stat.Name)

	mapUsage := newMapInterfaceNameUsage(nsInterfaces)
	assert.True(t, mapUsage.isTruncated())
	assert.Equal(t, 3, len(mapUsage.notTruncated()), "en0, gif0 and stf0")
}
