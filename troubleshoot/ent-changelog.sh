
#!/bin/bash

# Replace with the commit SHA from the consolidated changelog PR on the CE main branch
COMMIT_SHA="bd0c5448a527f825308a97df4cd6615dda5d211c"

# List the ENT-only patch release versions that share this changelog update
VERSIONS=(
    "2.0.4"
    "1.22.12"
    # Add other versions here, e.g., "1.17.5", "1.18.2"
)

# Ensure local repository is synced
git fetch origin main

for version in "${VERSIONS[@]}"; do
    TAG_NAME="ent-changelog-${version}"
    echo "Creating tag: ${TAG_NAME} at commit ${COMMIT_SHA}"
    
    # Create an annotated tag (recommended) pointing to the specified commit
    git tag -a "${TAG_NAME}" "${COMMIT_SHA}" -m "Dummy changelog tag for Enterprise-only patch release ${version}"
    
    # Push the tag to the remote repository
    # git push origin "${TAG_NAME}"
done

echo "Successfully created and pushed all dummy tags."
