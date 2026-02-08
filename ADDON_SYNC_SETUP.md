# Home Assistant Addon Auto-Sync Setup

This document explains how the automatic synchronization between the xbslink-ng repository and the Home Assistant addon repository works.

## Versioning

The two projects use **independent version schemes**:

- **xbslink-ng** (this repo) uses **semver** (`vMAJOR.MINOR.PATCH`, e.g., `v0.1.0`). Tags follow this format.
- **HA addon** uses **CalVer** (`YYYY.M.PATCH`, e.g., `2026.2.0`). The addon version increments automatically:
  - When a new xbslink-ng binary is pulled in (auto-bumps via workflow)
  - For addon-specific changes (config schema, Dockerfile, rootfs, etc.)

The xbslink-ng binary version used by the addon is tracked in the Dockerfile's `XBSLINK_VERSION` arg — it is **not** the same as the addon version.

## Overview

When a new release is published in this repository, it automatically triggers an update in the [home-assistant-addons](https://github.com/jchadwick/home-assistant-addons) repository, which will:

1. Update the `XBSLINK_VERSION` in the Dockerfile to the new xbslink-ng release
2. Auto-bump the addon's CalVer version in config.yaml
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
xbslink-ng Release Published (e.g., v0.1.0)
         ↓
notify-addon-release.yaml triggers
         ↓
Sends repository_dispatch event to home-assistant-addons
         ↓
update-xbslink-version.yaml triggers
         ↓
Updates Dockerfile XBSLINK_VERSION to v0.1.0
Bumps addon CalVer (e.g., 2026.2.0 → 2026.2.1)
         ↓
Commits and pushes changes
         ↓
builder.yaml detects changes and builds new addon
```

### Manual Trigger

You can also manually trigger the addon update from the home-assistant-addons repository:

1. Go to Actions → Update XBSLink-NG Version
2. Click "Run workflow"
3. Enter the version tag (e.g., `v0.1.0`)
4. Click "Run workflow"

## Files Involved

### xbslink-ng repository
- `.github/workflows/notify-addon-release.yaml` - Dispatches update event on release

### home-assistant-addons repository
- `.github/workflows/update-xbslink-version.yaml` - Receives dispatch, updates binary version, bumps addon CalVer
- `xbslink-ng/Dockerfile` - Contains `ARG XBSLINK_VERSION=vX.X.X` (xbslink-ng binary version, semver)
- `xbslink-ng/config.yaml` - Contains `version: "YYYY.M.PATCH"` (addon version, CalVer)

## Testing

To test the workflow without creating a real release:

1. Go to home-assistant-addons repository
2. Actions → Update XBSLink-NG Version → Run workflow
3. Enter a test version like `v0.0.2`
4. Verify the Dockerfile is updated and the addon CalVer was bumped

## Troubleshooting

- **Dispatch not triggering**: Check that `ADDON_DISPATCH_TOKEN` is set correctly
- **Permission errors**: Ensure the PAT has `repo` scope
- **Version format**: Dockerfile uses semver with `v` prefix (v0.1.0), config.yaml uses CalVer without prefix (2026.2.0)
