TSV_IN="API_ENDPOINTS.tsv"
HEADER="endpoint_header.mdx"
MASTER="endpoints_master.mdx"

awk -F"\t" 'BEGIN{OFS="|"}{print $1, "[\["$3"\] " $2"]("$4")", $7, $8 ;}' > $MASTER < "$TSV_IN"

(cat $HEADER; grep acl:read endpoints_master.mdx) > endpoints_acl_read.mdx
(cat $HEADER; grep acl:write endpoints_master.mdx) > endpoints_acl_write.mdx
(cat $HEADER; grep agent:read endpoints_master.mdx) > endpoints_agent_read.mdx
(cat $HEADER; grep agent:write endpoints_master.mdx) > endpoints_agent_write.mdx
(cat $HEADER; grep event:read endpoints_master.mdx) > endpoints_event_read.mdx
(cat $HEADER; grep event:write endpoints_master.mdx) > endpoints_event_write.mdx
(cat $HEADER; grep intentions:read endpoints_master.mdx) > endpoints_intentions_read.mdx
(cat $HEADER; grep intentions:write endpoints_master.mdx) > endpoints_intentions_write.mdx
(cat $HEADER; grep key:read endpoints_master.mdx) > endpoints_key_read.mdx
(cat $HEADER; grep key:write endpoints_master.mdx) > endpoints_key_write.mdx
(cat $HEADER; grep keyring:read endpoints_master.mdx) > endpoints_keyring_read.mdx
(cat $HEADER; grep keyring:write endpoints_master.mdx) > endpoints_keyring_write.mdx
(cat $HEADER; grep node:read endpoints_master.mdx) > endpoints_node_read.mdx
(cat $HEADER; grep node:write endpoints_master.mdx) > endpoints_node_write.mdx
(cat $HEADER; grep operator:read endpoints_master.mdx) > endpoints_operator_read.mdx
(cat $HEADER; grep operator:write endpoints_master.mdx) > endpoints_operator_write.mdx
(cat $HEADER; grep query:read endpoints_master.mdx) > endpoints_query_read.mdx
(cat $HEADER; grep query:write endpoints_master.mdx) > endpoints_query_write.mdx
(cat $HEADER; grep service:read endpoints_master.mdx) > endpoints_service_read.mdx
(cat $HEADER; grep service:write endpoints_master.mdx) > endpoints_service_write.mdx
(cat $HEADER; grep session:read endpoints_master.mdx) > endpoints_session_read.mdx
(cat $HEADER; grep session:write endpoints_master.mdx) > endpoints_session_write.mdx
