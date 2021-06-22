|                | service                 | connect (enterprise)    | ingress (enterprise)    | node                   | query           | addr                         |
|----------------|-------------------------|-------------------------|-------------------------|------------------------|-----------------|------------------------------|
| TypeSOA        | Supported               | Supported               | Supported               | Supported              | Supported       | Supported                    |
| TypeNS         | Supported               | Supported               | Supported               | Supported              | Supported       | Supported                    |
| TypeAXFR       | Not Implemented         | Not Implemented         | Not Implemented         | Not Implemented        | Not Implemented | Not Implemented              |
| TypeA/TypeAAAA | Supported               | Supported               | Supported               | Supported              |                 | Supported                    |
| TypeANY        | Supported (return A)    | Supported (return A)    | Supported (return A)    | Supported              |                 | Supported (return A)         |
| TypeCNAME      | Supported (node cname)  | Supported (node cname)  | Supported (node cname)  | Supported (node cname) |                 | return empty with A as extra |
| TypeOPT        | Supported (node OPT)    | Supported (node OPT)    | Supported (node OPT)    | Supported (node OPT)   |                 | return empty with A as extra |
| TypePTR        | Supported (node PTR)    | Supported (node PTR)    | Supported (node PTR)    | Supported (node PTR)   |                 | return empty with A as extra |
| TypeSRV        | Supported (service SRV) | Supported (service SRV) | Supported (service SRV) | No error but empty     |                 | return empty with A as extra |
| TypeTXT        | Answer A record (????)  | Answer A record (????)  | Answer A record (????)  | Supported              |                 | return empty with A as extra |