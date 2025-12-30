#!/bin/bash

echo "============================================"
echo " Cerberus Protocol - GitHub Deploy Helper"
echo "============================================"
echo ""

# Check if gh CLI is available
if command -v gh &> /dev/null; then
    echo "GitHub CLI detected! Creating repository..."
    echo ""

    gh repo create cerberus-protocol --public --source=. --remote=origin --push --description "AI-powered e-commerce product validation and scaling tool"

    if [ $? -eq 0 ]; then
        echo ""
        echo "✓ Repository created and code pushed successfully!"
        echo ""
        echo "View your repository at: https://github.com/YOUR_USERNAME/cerberus-protocol"
        echo ""
    else
        echo ""
        echo "✗ Failed to create repository. Please check your GitHub CLI authentication."
        echo "Run: gh auth login"
        echo ""
    fi
else
    echo "GitHub CLI not found. Please follow manual steps:"
    echo ""
    echo "1. Install GitHub CLI from: https://cli.github.com/"
    echo "   OR"
    echo "2. Create repository manually:"
    echo "   - Go to https://github.com/new"
    echo "   - Repository name: cerberus-protocol"
    echo "   - Click 'Create repository'"
    echo ""
    echo "3. Then run:"
    echo "   git remote add origin https://github.com/YOUR_USERNAME/cerberus-protocol.git"
    echo "   git branch -M main"
    echo "   git push -u origin main"
    echo ""
fi
