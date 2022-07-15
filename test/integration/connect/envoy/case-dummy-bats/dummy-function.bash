function dummyFunction {
  local LOCAL_VAR=$1
  echo $LOCAL_VAR $COMMON_VAR
}

function curlFunction {
  STATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" https://www.google.com)
  echo $STATUS_CODE
}

function jqFunction {
  INPUT_RAW_JSON=$1
  KEY_TO_FIND=$2
  RESULT=$(echo $INPUT_RAW_JSON | jq .$KEY_TO_FIND)
  echo $RESULT
}
