
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

git diff --name-only | xargs $GOIMPORTS -local "github.com/hashicorp/consul" -w

make --always-make proto
	   
