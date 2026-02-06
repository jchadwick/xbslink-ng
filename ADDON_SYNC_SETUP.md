# Home Assistant Addon Auto-Sync Setup

This document explains how the automatic synchronization between the xbslink-ng repository and the Home Assistant addon repository works.

## Overview

When a new release is published in this repository, it automatically triggers an update in the [home-assistant-addons](https://github.com/jchadwick/home-assistant-addons) repository, which will:

1. Update the `XBSLINK_VERSION` in the Dockerfile
2. Update the `version` field in config.yaml
3. Commit and push the changes
4. Trigger the addon build workflow automatically

## Setup Instructions

### 1. Create a Personal Access Token (PAT)

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Give it a descriptive name like "xbslink-ng addon sync"
4. Select the following scopes:
   - `repo` (Full control of private repositories)
5. Click "Generate token"
6. **Copy the token immediately** (you won't be able to see it again)

### 2. Add the Token to xbslink-ng Repository

1. Go to the xbslink-ng repository settings
2. Navigate to Secrets and variables → Actions
3. Click "New repository secret"
4. Name: `ADDON_DISPATCH_TOKEN`
5. Value: Paste the PAT you created in step 1
6. Click "Add secret"

## How It Works

### Release Flow

```
xbslink-ng Release Published
         ↓
notify-addon-release.yaml triggers
         ↓
Sends repository_dispatch event to home-assistant-addons
         ↓
update-xbslink-version.yaml triggers
         ↓
Updates Dockerfile and config.yaml
         ↓
Commits and pushes changes
         ↓
builder.yaml detects changes and builds new addon
```

### Manual Trigger

You can also manually trigger the addon update from the home-assistant-addons repository:

1. Go to Actions → Update XBSLink-NG Version
2. Click "Run workflow"
3. Enter the version tag (e.g., `v0.0.2`)
4. Click "Run workflow"

## Files Involved

### xbslink-ng repository
- `.github/workflows/notify-addon-release.yaml` - Dispatches update event on release

### home-assistant-addons repository
- `.github/workflows/update-xbslink-version.yaml` - Receives dispatch and updates version
- `xbslink-ng/Dockerfile` - Contains `ARG XBSLINK_VERSION=vX.X.X`
- `xbslink-ng/config.yaml` - Contains `version: "X.X.X"`

## Testing

To test the workflow without creating a real release:

1. Go to home-assistant-addons repository
2. Actions → Update XBSLink-NG Version → Run workflow
3. Enter a test version like `v0.0.1`
4. Verify the files are updated correctly

## Troubleshooting

- **Dispatch not triggering**: Check that `ADDON_DISPATCH_TOKEN` is set correctly
- **Permission errors**: Ensure the PAT has `repo` scope
- **Version mismatch**: The Dockerfile uses versions with 'v' prefix (v0.0.2), while config.yaml uses without (0.0.2)
