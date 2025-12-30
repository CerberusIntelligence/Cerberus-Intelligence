@echo off
echo ============================================
echo  Cerberus Protocol - GitHub Deploy Helper
echo ============================================
echo.

REM Check if gh CLI is available
where gh >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo GitHub CLI detected! Creating repository...
    echo.
    gh repo create cerberus-protocol --public --source=. --remote=origin --push --description "AI-powered e-commerce product validation and scaling tool"
    if %ERRORLEVEL% EQU 0 (
        echo.
        echo ✓ Repository created and code pushed successfully!
        echo.
        echo View your repository at: https://github.com/YOUR_USERNAME/cerberus-protocol
        echo.
    ) else (
        echo.
        echo ✗ Failed to create repository. Please check your GitHub CLI authentication.
        echo Run: gh auth login
        echo.
    )
) else (
    echo GitHub CLI not found. Please follow manual steps:
    echo.
    echo 1. Install GitHub CLI from: https://cli.github.com/
    echo    OR
    echo 2. Create repository manually:
    echo    - Go to https://github.com/new
    echo    - Repository name: cerberus-protocol
    echo    - Click 'Create repository'
    echo.
    echo 3. Then run:
    echo    git remote add origin https://github.com/YOUR_USERNAME/cerberus-protocol.git
    echo    git branch -M main
    echo    git push -u origin main
    echo.
)

pause
