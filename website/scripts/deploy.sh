#!/usr/bin/env bash
set -e

PROJECT="consul"
PROJECT_URL="www.consul.io"
FASTLY_SERVICE_ID="7GrxRJP3PVBuqQbyxYQ0MV"
REDIRECTS_FILE="./source/redirects.txt"
EXTERNAL_REDIRECTS_DICT_ID="6jIf0fzbdc8TQerhjkPBc4"
RELATIVE_REDIRECTS_DICT_ID="431fGfpCzjVIESAwTBVjlf"

# This function posts a dictionary of key-value pairs for redirects to Fastly
function post_redirects {
    # Arguments:
    #   $1 - jq_query
    #   $2 - dictionary_id
    #   $3 - jq_args
    #
    # Returns:
    #   0 - success
    #   * - failure

    declare -a arr=("${!3}")

    # Do not post empty items (the API gets sad)
     if [ "${#arr[@]}" -ne 0 ]; then
         json="$(jq "${arr[@]}" "$1" <<<'{"items": []}')"

        # Post the JSON body
        curl \
        --request "PATCH" \
        --header "Fastly-Key: $FASTLY_API_KEY" \
        --header "Content-type: application/json" \
        --header "Accept: application/json" \
        --data "$json"\
        "https://api.fastly.com/service/$FASTLY_SERVICE_ID/dictionary/$2/items"
     fi
}

# Ensure the proper AWS environment variables are set
if [ -z "$AWS_ACCESS_KEY_ID" ]; then
  echo "Missing AWS_ACCESS_KEY_ID!"
  exit 1
fi

if [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
  echo "Missing AWS_SECRET_ACCESS_KEY!"
  exit 1
fi

# Ensure the proper Fastly keys are set
if [ -z "$FASTLY_API_KEY" ]; then
  echo "Missing FASTLY_API_KEY!"
  exit 1
fi

# Ensure we have s3cmd installed
if ! command -v "s3cmd" >/dev/null 2>&1; then
  echo "Missing s3cmd!"
  exit 1
fi

# Ensure the proper analytics Segment ID is set
if [ "$ENV" != "production" ]; then
  echo "ENV is not set to production for Segment Analytics!"
  exit 1
fi

# Get the parent directory of where this script is and cd there
DIR="$(cd "$(dirname "$(readlink -f "$0")")/.." && pwd)"

# Delete any .DS_Store files for our OS X friends.
find "$DIR" -type f -name '.DS_Store' -delete

# Upload the files to S3 - we disable mime-type detection by the python library
# and just guess from the file extension because it's surprisingly more
# accurate, especially for CSS and javascript. We also tag the uploaded files
# with the proper Surrogate-Key, which we will later purge in our API call to
# Fastly.
if [ -z "$NO_UPLOAD" ]; then
  echo "Uploading to S3..."

  # Check that the site has been built
  if [ ! -d "$DIR/build" ]; then
    echo "Missing compiled website! Run 'make build' to compile!"
    exit 1
  fi

  # Set browser-side cache-control to ~4h, but tell Fastly to cache for much
  # longer. We manually purge the Fastly cache, so setting it to a year is more
  # than fine.
  s3cmd \
    --quiet \
    --delete-removed \
    --guess-mime-type \
    --no-mime-magic \
    --acl-public \
    --recursive \
    --add-header="Cache-Control: max-age=14400" \
    --add-header="x-amz-meta-surrogate-control: max-age=31536000, stale-white-revalidate=86400, stale-if-error=604800" \
    --add-header="x-amz-meta-surrogate-key: site-$PROJECT" \
    sync "$DIR/build/" "s3://hc-sites/$PROJECT/latest/"

  # The s3cmd guessed mime type for text files is often wrong. This is
  # problematic for some assets, so force their mime types to be correct.
  echo "Overriding javascript mime-types..."
  s3cmd \
    --mime-type="application/javascript" \
    --add-header="Cache-Control: max-age=31536000" \
    --exclude "*" \
    --include "*.js" \
    --recursive \
    modify "s3://hc-sites/$PROJECT/latest/"

  echo "Overriding css mime-types..."
  s3cmd \
    --mime-type="text/css" \
    --add-header="Cache-Control: max-age=31536000" \
    --exclude "*" \
    --include "*.css" \
    --recursive \
    modify "s3://hc-sites/$PROJECT/latest/"

  echo "Overriding svg mime-types..."
  s3cmd \
    --mime-type="image/svg+xml" \
    --add-header="Cache-Control: max-age=31536000" \
    --exclude "*" \
    --include "*.svg" \
    --recursive \
    modify "s3://hc-sites/$PROJECT/latest/"
fi

# Add redirects if they exist
# The redirects file is in the source/ directory
if [ -z "$NO_REDIRECTS" ] || [ ! test -f $REDIRECTS_FILE ]; then
  echo "Adding redirects..."
  fields=()
  while read -r line; do
    [[ "$line" =~ ^#.* ]] && continue
    [[ -z "$line" ]] && continue

    # Read fields
    IFS=" " read -ra parts <<<"$line"
    fields+=("${parts[@]}")
  done < $REDIRECTS_FILE

  # Check we have pairs
  if [ $((${#fields[@]} % 2)) -ne 0 ]; then
    echo "Bad redirects (not an even number)!"
    exit 1
  fi

  # Check we don't have more than 1000 entries (yes, it says 2000 below, but that
  # is because we've split into multiple lines).
  if [ "${#fields}" -gt 2000 ]; then
    echo "More than 1000 entries!"
    exit 1
  fi

  # Validations
  for field in "${fields[@]}"; do
    if [ "${#field}" -gt 256 ]; then
      echo "'$field' is > 256 characters!"
      exit 1
    fi
  done

  # Build the payload for single-request updates.
  jq_args_external=()
  jq_query_external="."
  jq_args_relative=()
  jq_query_relative="."
  for (( i=0; i<${#fields[@]}; i+=2 )); do
    # if the redirect is external, add the entries to the external list
    if [[ "${fields[i+1]}" =~ http[s]*:\/\/ ]]; then
        external_original="${fields[i]}"
        external_redirect="${fields[i+1]}"
        echo "Redirecting external ${external_original} -> ${external_redirect}"
        # The key and value indexes are for the whole redirects file so it may not be contiguous
        # This doesn't matter at the end as long as the query matches the same key/value items
        jq_args_external+=(--arg "key$((i/2))" "${external_original}")
        jq_args_external+=(--arg "value$((i/2))" "${external_redirect}")
        jq_query_external+="| .items |= (. + [{op: \"upsert\", item_key: \$key$((i/2)), item_value: \$value$((i/2))}])"
    else
        relative_original="${fields[i]}"
        relative_redirect="${fields[i+1]}"
        echo "Redirecting relative ${relative_original} -> ${relative_redirect}"
        # The key and value indexes are for the whole redirects file so it may not be contiguous
        # This doesn't matter at the end as long as the query matches the same key/value items
        jq_args_relative+=(--arg "key$((i/2))" "${relative_original}")
        jq_args_relative+=(--arg "value$((i/2))" "${relative_redirect}")
        jq_query_relative+="| .items |= (. + [{op: \"upsert\", item_key: \$key$((i/2)), item_value: \$value$((i/2))}])"
    fi
  done
    post_redirects "$jq_query_external" $EXTERNAL_REDIRECTS_DICT_ID jq_args_external[@]
    post_redirects "$jq_query_relative" $RELATIVE_REDIRECTS_DICT_ID jq_args_relative[@]
fi

# Perform a purge of the surrogate key.
if [ -z "$NO_PURGE" ]; then
  echo "Purging Fastly cache..."
  curl \
    --request "POST" \
    --header "Accept: application/json" \
    --header "Fastly-Key: $FASTLY_API_KEY" \
    --header "Fastly-Soft-Purge: 1" \
    "https://api.fastly.com/service/$FASTLY_SERVICE_ID/purge/site-$PROJECT"
fi

# Warm the cache with recursive wget.
if [ -z "$NO_WARM" ]; then
  echo "Warming Fastly cache..."
  echo ""
  echo "If this step fails, there are likely missing or broken assets or links"
  echo "on the website. Run the following command manually on your laptop, and"
  echo "search for \"ERROR\" in the output:"
  echo ""
  echo "wget --recursive --delete-after https://$PROJECT_URL/"
  echo ""
  wget \
    --delete-after \
    --level inf \
    --no-directories \
    --no-host-directories \
    --no-verbose \
    --page-requisites \
    --recursive \
    --spider \
    "https://$PROJECT_URL/"
fi
