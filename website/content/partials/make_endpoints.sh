TSV_IN="API_ENDPOINTS.tsv"
HEADER="endpoint_header.mdx"
ALL="endpoints_all.mdx"

generate_file() {
  case ""$2"" in
  "read")
    (cat $HEADER; pcregrep $1:$2 $ALL) > endpoints_$1_$2.mdx
    ;;
  "write")
    (cat $HEADER; pcregrep $1':(read|list|write)' $ALL)  > endpoints_$1_$2.mdx
    ;;
  esac
}

awk -F"\t" 'BEGIN{OFS="|"}{print $1, "[\["$3"\] " $2"]("$4")", $7, $8 ;}' > $ALL < "$TSV_IN"

for RESOURCE in acl agent event intentions key keyring node operator query service session
do
    # todo add list
    for ACCESS in read write
    do
	generate_file $RESOURCE $ACCESS
    done
done
