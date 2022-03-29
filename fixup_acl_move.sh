
GOIMPORTS=~/go/bin/goimports

CHANGED=(EnterpriseMeta PartitionOrDefault IsDefaultPartition NamespaceOrDefault NewEnterpriseMetaWithPartition EqualPartitions)

DIRS=(agent command proto)

for dir in "${DIRS[@]}"
  do 
      echo "CD to $dir"
      pushd $dir
      for s in "${CHANGED[@]}"
      do
	  REWRITE='structs.'$s' -> acl.'$s
	  echo "REPL $REWRITE"
	  gofmt -w -r="$REWRITE" .
      done
      popd
done

find . -name \*.go | xargs fgrep 'acl.' -l | xargs $GOIMPORTS -local "github.com/hashicorp/consul" -w

make --always-make proto


go get google.golang.org/protobuf/reflect/protoreflect
go get google.golang.org/protobuf/types/known/structpb
go get google.golang.org/protobuf/runtime/protoimpl
go get github.com/hashicorp/consul/agent/xds
go get github.com/hashicorp/consul/agent/structs
go get google.golang.org/protobuf

