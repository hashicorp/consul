# This script is a helper of migration to gopsutil v2 using gorename
# 
# go get golang.org/x/tools/cmd/gorename

IFS=$'\n'

## Part 1. rename Functions to pass golint.  ex) cpu.CPUTimesStat -> cpu.TimesStat

#
# Note:
#   process has IOCounters() for file IO, and also NetIOCounters() for Net IO.
#   This scripts replace process.NetIOCounters() to IOCounters().
#   So you need hand-fixing process.

TARGETS=`cat <<EOF
CPUTimesStat -> TimesStat
CPUInfoStat -> InfoStat
CPUTimes -> Times
CPUInfo -> Info
CPUCounts -> Counts
CPUPercent -> Percent
DiskUsageStat -> UsageStat
DiskPartitionStat -> PartitionStat
DiskIOCountersStat -> IOCountersStat
DiskPartitions -> Partitions
DiskIOCounters -> IOCounters
DiskUsage -> Usage
HostInfoStat -> InfoStat
HostInfo -> Info
GetVirtualization -> Virtualization
GetPlatformInformation -> PlatformInformation
LoadAvgStat -> AvgStat
LoadAvg -> Avg
NetIOCountersStat -> IOCountersStat
NetConnectionStat -> ConnectionStat
NetProtoCountersStat -> ProtoCountersStat
NetInterfaceAddr -> InterfaceAddr
NetInterfaceStat -> InterfaceStat
NetFilterStat -> FilterStat
NetInterfaces -> Interfaces
getNetIOCountersAll -> getIOCountersAll
NetIOCounters -> IOCounters
NetIOCountersByFile -> IOCountersByFile
NetProtoCounters -> ProtoCounters
NetFilterCounters -> FilterCounters
NetConnections -> Connections
NetConnectionsPid -> ConnectionsPid
Uid -> UID
Id -> ID
convertCpuTimes -> convertCPUTimes
EOF`

for T in $TARGETS
do
  echo $T
  gofmt -w -r "$T" ./*.go
done


###### Part 2  rename JSON key name
## Google JSOn style
## https://google.github.io/styleguide/jsoncstyleguide.xml

sed -i "" 's/guest_nice/guestNice/g' cpu/*.go
sed -i "" 's/vendor_id/vendorId/g' cpu/*.go
sed -i "" 's/physical_id/physicalId/g' cpu/*.go
sed -i "" 's/model_name/modelName/g' cpu/*.go
sed -i "" 's/cache_size/cacheSize/g' cpu/*.go
sed -i "" 's/core_id/coreId/g' cpu/*.go

sed -i "" 's/inodes_total/inodesTotal/g' disk/*.go
sed -i "" 's/inodes_used/inodesUsed/g' disk/*.go
sed -i "" 's/inodes_free/inodesFree/g' disk/*.go
sed -i "" 's/inodes_used_percent/inodesUsedPercent/g' disk/*.go
sed -i "" 's/read_count/readCount/g' disk/*.go
sed -i "" 's/write_count/writeCount/g' disk/*.go
sed -i "" 's/read_bytes/readBytes/g' disk/*.go
sed -i "" 's/write_bytes/writeBytes/g' disk/*.go
sed -i "" 's/read_time/readTime/g' disk/*.go
sed -i "" 's/write_time/writeTime/g' disk/*.go
sed -i "" 's/io_time/ioTime/g' disk/*.go
sed -i "" 's/serial_number/serialNumber/g' disk/*.go
sed -i "" 's/used_percent/usedPercent/g' disk/*.go
sed -i "" 's/inodesUsed_percent/inodesUsedPercent/g' disk/*.go

sed -i "" 's/total_cache/totalCache/g' docker/*.go
sed -i "" 's/total_rss_huge/totalRssHuge/g' docker/*.go
sed -i "" 's/total_rss/totalRss/g' docker/*.go
sed -i "" 's/total_mapped_file/totalMappedFile/g' docker/*.go
sed -i "" 's/total_pgpgin/totalPgpgin/g' docker/*.go
sed -i "" 's/total_pgpgout/totalPgpgout/g' docker/*.go
sed -i "" 's/total_pgfault/totalPgfault/g' docker/*.go
sed -i "" 's/total_pgmajfault/totalPgmajfault/g' docker/*.go
sed -i "" 's/total_inactive_anon/totalInactiveAnon/g' docker/*.go
sed -i "" 's/total_active_anon/totalActiveAnon/g' docker/*.go
sed -i "" 's/total_inactive_file/totalInactiveFile/g' docker/*.go
sed -i "" 's/total_active_file/totalActiveFile/g' docker/*.go
sed -i "" 's/total_unevictable/totalUnevictable/g' docker/*.go
sed -i "" 's/mem_usage_in_bytes/memUsageInBytes/g' docker/*.go
sed -i "" 's/mem_max_usage_in_bytes/memMaxUsageInBytes/g' docker/*.go
sed -i "" 's/memory.limit_in_bytes/memoryLimitInBbytes/g' docker/*.go
sed -i "" 's/memory.failcnt/memoryFailcnt/g' docker/*.go
sed -i "" 's/mapped_file/mappedFile/g' docker/*.go
sed -i "" 's/container_id/containerID/g' docker/*.go
sed -i "" 's/rss_huge/rssHuge/g' docker/*.go
sed -i "" 's/inactive_anon/inactiveAnon/g' docker/*.go
sed -i "" 's/active_anon/activeAnon/g' docker/*.go
sed -i "" 's/inactive_file/inactiveFile/g' docker/*.go
sed -i "" 's/active_file/activeFile/g' docker/*.go
sed -i "" 's/hierarchical_memory_limit/hierarchicalMemoryLimit/g' docker/*.go

sed -i "" 's/boot_time/bootTime/g' host/*.go
sed -i "" 's/platform_family/platformFamily/g' host/*.go
sed -i "" 's/platform_version/platformVersion/g' host/*.go
sed -i "" 's/virtualization_system/virtualizationSystem/g' host/*.go
sed -i "" 's/virtualization_role/virtualizationRole/g' host/*.go

sed -i "" 's/used_percent/usedPercent/g' mem/*.go

sed -i "" 's/bytes_sent/bytesSent/g' net/*.go
sed -i "" 's/bytes_recv/bytesRecv/g' net/*.go
sed -i "" 's/packets_sent/packetsSent/g' net/*.go
sed -i "" 's/packets_recv/packetsRecv/g' net/*.go
sed -i "" 's/conntrack_count/conntrackCount/g' net/*.go
sed -i "" 's/conntrack_max/conntrackMax/g' net/*.go

sed -i "" 's/read_count/readCount/g' process/*.go
sed -i "" 's/write_count/writeCount/g' process/*.go
sed -i "" 's/read_bytes/readBytes/g' process/*.go
sed -i "" 's/write_bytes/writeBytes/g' process/*.go
sed -i "" 's/shared_clean/sharedClean/g' process/*.go
sed -i "" 's/shared_dirty/sharedDirty/g' process/*.go
sed -i "" 's/private_clean/privateClean/g' process/*.go
sed -i "" 's/private_dirty/privateDirty/g' process/*.go
