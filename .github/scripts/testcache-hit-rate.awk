#!/usr/bin/env -S awk -f

BEGIN {
  lookups = 0
  saves = 0
}
/: save test ID/ { 
  saves++
}
/: test ID/ {
  lookups++
}
END {
  printf("testcache hit rate: %.2f %%", (lookups - saves) * 100 / lookups)
}