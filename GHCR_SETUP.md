# GitHub Container Registry Setup

This document explains how to configure the GitHub Container Registry (GHCR) package visibility for xbslink-ng.

## The Issue

By default, GitHub Container Registry packages are **private**. This means users cannot pull the Docker image without authentication:

```bash
docker pull ghcr.io/jchadwick/xbslink-ng:latest
# Error: denied: denied
```

## Solution

You need to make the package **public** so anyone can pull it. You only need to do this once.

### Option 1: Make it Public via GitHub Web UI (Recommended)

1. Go to: https://github.com/jchadwick?tab=packages
2. Click on the **"xbslink-ng"** package
3. Click **"Package settings"** (gear icon in the top right)
4. Scroll down to the **"Danger Zone"** section
5. Click **"Change visibility"**
6. Select **"Public"**
7. Type the package name to confirm
8. Click **"I understand, change package visibility"**

### Option 2: Make it Public via GitHub CLI

```bash
gh api \
  --method PATCH \
  -H "Accept: application/vnd.github+json" \
  /user/packages/container/xbslink-ng \
  -f visibility='public'
```

Note: This requires your GitHub CLI to be authenticated with a token that has `write:packages` scope.

## Automated Workflow

The GitHub Actions workflow includes a step that attempts to automatically make the package public after pushing. However, this has limitations:

- The default `GITHUB_TOKEN` may not have sufficient permissions
- The step is marked as `continue-on-error: true` so it won't fail the build
- If it fails, you'll need to make it public manually (one-time)

Once the package is public, it will remain public for all future pushes.

## Verifying It Works

After making it public, test it:

```bash
docker pull ghcr.io/jchadwick/xbslink-ng:latest
docker run --rm ghcr.io/jchadwick/xbslink-ng:latest version
```

## Linking Package to Repository

To display the package on your repository page:

1. Go to the package settings (same as above)
2. Under **"Danger Zone"**, find **"Connect repository"**
3. Select `jchadwick/xbslink-ng`
4. This will show the package in your repository's sidebar

## Available Tags

After setup, the following tags are available:

- `latest` - Latest release from main branch
- `dev-latest` - Latest development build
- `main-<sha>` - Specific commit (e.g., `main-6573c45`)
