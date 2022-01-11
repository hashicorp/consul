TSV_IN="API_ENDPOINTS.tsv"
HEADER="endpoint_header.mdx"
MASTER="endpoints_master.mdx"

generate_file() {
    (cat $HEADER; grep $1:$2 $MASTER) > endpoints_$1_$2.mdx
}

awk -F"\t" 'BEGIN{OFS="|"}{print $1, "[\["$3"\] " $2"]("$4")", $7, $8 ;}' > $MASTER < "$TSV_IN"

for RESOURCE in acl agent event intentions key keyring node operator query service session
do
    # todo add list
    for ACCESS in read write 
    do
	generate_file $RESOURCE $ACCESS 
    done
done
