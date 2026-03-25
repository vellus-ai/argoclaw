# Distribution Guide

## Distribution Models in ArgoClaw

### 1. Publish to Current Instance (Primary)

Register skill directly in the running ArgoClaw instance:

```
publish_skill(path: "~/.argoclaw/skills-store/<name>")
```

- Copies files to managed store: `~/.argoclaw/skills-store/<slug>/<version>/  (Docker: /app/.argoclaw/skills-store/)`
- Registers in `skills` table (`is_system=false`, `visibility='public'`)
- Scans + reports missing dependencies
- Auto-increments version only if SKILL.md content (SHA-256) changes

### 2. Upload via Admin UI

Package skill as ZIP, then upload via ArgoClaw admin dashboard (`/skills` page):

```bash
scripts/package_skill.py ~/.argoclaw/skills-store/<name>
# → creates <name>.zip
```

Upload at: **Admin UI → Skills → Upload skill**

Use case: distributing skills to other ArgoClaw instances without direct filesystem access.

### 3. Bundled Skills (Image-level)

Skills placed in the `skills/` directory of the repo are bundled into the Docker image:

```
skills/
└── my-skill/
    └── SKILL.md
```

Rebuild required: `docker compose up -d --build`

Bundled skills are seeded automatically on gateway startup. They have lowest priority — user-uploaded skills with same slug override them.

Use case: ship default skills with every ArgoClaw deployment.

## Version Management

ArgoClaw manages versions automatically via content hash:

| Scenario | Result |
|----------|--------|
| First publish | `version = 1` |
| Re-publish, content unchanged | No-op (version stays) |
| Re-publish, SKILL.md changed | `version += 1` |
| Upload same slug via UI | Version bumped, new files copied |

Do NOT manually set `version` in SKILL.md frontmatter — it has no effect on ArgoClaw's versioning.

## Dependency Handling

After publishing, ArgoClaw scans for missing Python/Node deps automatically.

If deps are missing (`status = archived`):
1. View missing deps in Admin UI → Skills → skill row
2. Click "Install" per-dep, or install manually via exec tool:
   ```bash
   pip3 install <pkg>
   npm install -g <pkg>
   ```
3. Skill auto-transitions to `status = active` after install

System packages (apk) require `ENABLE_PYTHON=true` and `doas` available in the image.

## Sharing Skills

To share a skill externally:

1. Package: `scripts/package_skill.py ~/.argoclaw/skills-store/<name>` → ZIP file
2. Share the ZIP — recipient uploads via Admin UI → Skills → Upload
3. Or contribute to `skills/` directory in the ArgoClaw repo for bundling
