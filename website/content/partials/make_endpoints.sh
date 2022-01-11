cat ~/Downloads/API\ Endpoints,\ Commands,\ \&\ ACLs\ -\ MAIN.tsv| awk -F"\t" 'BEGIN{OFS="|"}{print $1, "["$3 $2"]("$4")", $7, $8 ;}' > endpoints_master.mdx

(cat endpoint_header.mdx; grep acl:read endpoints_master.mdx) > endpoints_acl_read.mdx
(cat endpoint_header.mdx; grep acl:write endpoints_master.mdx) > endpoints_acl_write.mdx
(cat endpoint_header.mdx; grep agent:read endpoints_master.mdx) > endpoints_agent_read.mdx
(cat endpoint_header.mdx; grep agent:write endpoints_master.mdx) > endpoints_agent_write.mdx
(cat endpoint_header.mdx; grep event:read endpoints_master.mdx) > endpoints_event_read.mdx
(cat endpoint_header.mdx; grep event:write endpoints_master.mdx) > endpoints_event_write.mdx
(cat endpoint_header.mdx; grep intentions:read endpoints_master.mdx) > endpoints_intentions_read.mdx
(cat endpoint_header.mdx; grep intentions:write endpoints_master.mdx) > endpoints_intentions_write.mdx
(cat endpoint_header.mdx; grep key:read endpoints_master.mdx) > endpoints_key_read.mdx
(cat endpoint_header.mdx; grep key:write endpoints_master.mdx) > endpoints_key_write.mdx
(cat endpoint_header.mdx; grep keyring:read endpoints_master.mdx) > endpoints_keyring_read.mdx
(cat endpoint_header.mdx; grep keyring:write endpoints_master.mdx) > endpoints_keyring_write.mdx
(cat endpoint_header.mdx; grep node:read endpoints_master.mdx) > endpoints_node_read.mdx
(cat endpoint_header.mdx; grep node:write endpoints_master.mdx) > endpoints_node_write.mdx
(cat endpoint_header.mdx; grep operator:read endpoints_master.mdx) > endpoints_operator_read.mdx
(cat endpoint_header.mdx; grep operator:write endpoints_master.mdx) > endpoints_operator_write.mdx
(cat endpoint_header.mdx; grep query:read endpoints_master.mdx) > endpoints_query_read.mdx
(cat endpoint_header.mdx; grep query:write endpoints_master.mdx) > endpoints_query_write.mdx
(cat endpoint_header.mdx; grep service:read endpoints_master.mdx) > endpoints_service_read.mdx
(cat endpoint_header.mdx; grep service:write endpoints_master.mdx) > endpoints_service_write.mdx
(cat endpoint_header.mdx; grep session:read endpoints_master.mdx) > endpoints_session_read.mdx
(cat endpoint_header.mdx; grep session:write endpoints_master.mdx) > endpoints_session_write.mdx
