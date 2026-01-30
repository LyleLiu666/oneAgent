#!/bin/bash
set -e

mkdir -p docs/opensource/issues

fetch_repo() {
    REPO=$1
    NAME=$2
    echo "Fetching issues for $NAME ($REPO)..."
    
    # Use search query for sorting by reactions
    gh issue list --repo "$REPO" --search "is:issue is:open sort:reactions-plus" --limit 5 \
        --json number,title,body,url,createdAt \
        > "docs/opensource/issues/${NAME}_issues.json"
        
    cat > "docs/opensource/issues/${NAME}.md" <<EOF
# $NAME Top Issues (Open & High Reactions)

Fetched at: $(date)
Repo: https://github.com/$REPO

EOF

    if command -v jq &> /dev/null; then
        # Simple jq filter, just dumping body as string
        jq -r '.[] | "## [\(.title)](\(.url)) (#\(.number))\n**Created**: \(.createdAt)\n\n\(.body)\n\n---\n"' \
           "docs/opensource/issues/${NAME}_issues.json" >> "docs/opensource/issues/${NAME}.md"
    else
        echo "jq not found, saving raw JSON"
        cat "docs/opensource/issues/${NAME}_issues.json" >> "docs/opensource/issues/${NAME}.md"
    fi
    
    rm "docs/opensource/issues/${NAME}_issues.json"
}

fetch_repo "n8n-io/n8n" "n8n"
fetch_repo "labring/FastGPT" "FastGPT"
fetch_repo "langgenius/dify" "Dify"
fetch_repo "continuedev/continue" "Continue"
fetch_repo "Aider-AI/aider" "Aider"

echo "Done."
