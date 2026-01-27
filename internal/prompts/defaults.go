package prompts

// GetDefaultPrompts returns all default prompt templates as YAML strings
// These are written to disk when the prompts directory is first created
func GetDefaultPrompts() map[string]string {
	return map[string]string{
		"batch-metadata":        defaultBatchMetadata,
		"query-and-apply":       defaultQueryAndApply,
		"upload-files":          defaultUploadFiles,
		"lineage-relationships": defaultLineageRelationships,
		"conditional-metadata":  defaultConditionalMetadata,
	}
}

const defaultBatchMetadata = `name: batch-metadata
description: Set or delete metadata on multiple assets by hash
category: metadata
template: |
  # Batch Metadata Operations

  ## Purpose
  Set or delete metadata on multiple assets using their BLAKE3 hashes.

  ## Base URL
  {{base_url}}

  ## Discovering Available Queries
  Before writing your script, discover available query presets:
  GET {{base_url}}/api/queries

  This returns all available presets with their parameters. Use the appropriate
  preset based on your task requirements.

  ## API Endpoint
  POST /api/metadata/batch

  ## Request Format
  ` + "```" + `json
  {
    "operations": [
      {
        "hash": "<64-character-BLAKE3-hash>",
        "op": "set",
        "key": "<metadata-key>",
        "value": <any-json-value>
      },
      {
        "hash": "<64-character-BLAKE3-hash>",
        "op": "delete",
        "key": "<metadata-key>"
      }
    ],
    "processor": "<your-script-name>",
    "processor_version": "<version>"
  }
  ` + "```" + `

  ## Field Details
  - **hash**: 64-character hexadecimal BLAKE3 hash (lowercase)
  - **op**: Either "set" or "delete"
  - **key**: Metadata key name (max 256 characters)
  - **value**: Any JSON value - string, number, boolean, array, or object (max 10MB)
  - **processor**: Name identifying your script/tool (for audit trail)
  - **processor_version**: Version of your script (for audit trail)

  ## Response Format
  ` + "```" + `json
  {
    "success": true,
    "total": 3,
    "succeeded": 2,
    "failed": 1,
    "results": [
      {"hash": "abc123...", "success": true, "log_id": 42},
      {"hash": "def456...", "success": true, "log_id": 43},
      {"hash": "ghi789...", "success": false, "error": "ASSET_NOT_FOUND"}
    ]
  }
  ` + "```" + `

  ## Constraints
  - Maximum 100,000 operations per request
  - Metadata key: max 256 characters
  - Metadata value: max 10MB (10,485,760 bytes)
  - Hash must be exactly 64 hexadecimal characters (lowercase)
  - Operations are atomic per topic (all succeed or all fail within each topic)

  ## Error Codes
  | Code | Description |
  |------|-------------|
  | BATCH_TOO_MANY_OPERATIONS | More than 100,000 operations in request |
  | BATCH_INVALID_OPERATION | Invalid op field (must be "set" or "delete") |
  | BATCH_PARTIAL_FAILURE | Some operations failed (check results array) |
  | INVALID_HASH | Hash is not 64 hex characters |
  | ASSET_NOT_FOUND | No asset exists with this hash |
  | METADATA_KEY_TOO_LONG | Key exceeds 256 characters |
  | METADATA_VALUE_TOO_LONG | Value exceeds 10MB |

  ## Complete Example
  ` + "```" + `python
  import requests
  import json

  BASE_URL = "{{base_url}}"

  # Your data: list of {hash, key, value} to set
  data = [
      {"hash": "a1b2c3...", "key": "status", "value": "approved"},
      {"hash": "d4e5f6...", "key": "tags", "value": ["character", "rigged"]},
  ]

  # Build operations
  operations = [
      {"hash": item["hash"], "op": "set", "key": item["key"], "value": item["value"]}
      for item in data
  ]

  response = requests.post(
      f"{BASE_URL}/api/metadata/batch",
      json={
          "operations": operations,
          "processor": "my-batch-script",
          "processor_version": "1.0"
      }
  )

  result = response.json()
  print(f"Total: {result['total']}, Succeeded: {result['succeeded']}, Failed: {result['failed']}")

  if result['failed'] > 0:
      for r in result['results']:
          if not r['success']:
              print(f"Failed: {r['hash']} - {r.get('error', 'Unknown error')}")
  ` + "```" + `

  ## Script Requirements
  Your script should:
  1. Parse the user's data format (JSON, CSV, or custom)
  2. Validate each hash is 64 hex characters before sending
  3. Build the operations array with correct structure
  4. Send POST request with processor name and version
  5. Handle partial failures by checking each result
  6. Report success/failure counts and specific errors

  ---
  ## YOUR CONTEXT
  [Describe your data format and what metadata you want to set/delete]
`

const defaultQueryAndApply = `name: query-and-apply
description: Query assets and apply metadata to all matching results
category: metadata
template: |
  # Query and Apply Metadata

  ## Purpose
  Find assets matching criteria and apply metadata to all results in one operation.

  ## Base URL
  {{base_url}}

  ## Discovering Available Queries
  Before writing your script, discover available query presets:
  GET {{base_url}}/api/queries

  This returns all available presets with their parameters, including:
  - Basic queries (recent-imports, by-hash, large-files, by-extension, by-origin-name)
  - Metadata queries (without-metadata, with-metadata, by-processor)
  - Lineage queries (lineage, derived, orphans, roots-with-children)
  - Analytics queries (extension-summary, size-distribution, time-series)

  Use the appropriate preset based on your task requirements.

  ## Two Approaches

  ### Approach 1: Apply Directly (Recommended for simple cases)
  POST /api/metadata/apply

  This re-executes the query and applies metadata atomically per topic.

  ### Approach 2: Query First, Then Batch
  1. POST /api/query/:preset - Get matching assets
  2. POST /api/metadata/batch - Apply to results

  Use this when you need to filter or transform results before applying.

  ## Apply Endpoint Request Format
  POST /api/metadata/apply
  ` + "```" + `json
  {
    "query_preset": "<preset-name>",
    "query_params": {"param1": "value1", "limit": "1000"},
    "topics": ["my-topic"],
    "op": "set",
    "key": "file_type",
    "value": "3d-model",
    "processor": "categorizer-script",
    "processor_version": "1.0"
  }
  ` + "```" + `

  ## Apply Endpoint Response
  ` + "```" + `json
  {
    "success": true,
    "assets_matched": 150,
    "assets_updated": 150,
    "topics_affected": 1
  }
  ` + "```" + `

  ## Query Endpoint Request Format
  POST /api/query/:preset
  ` + "```" + `json
  {
    "topics": ["topic1", "topic2"],
    "params": {"ext": "glb", "limit": "100"}
  }
  ` + "```" + `

  ## Query Endpoint Response
  ` + "```" + `json
  {
    "results": [
      {
        "asset_id": "abc123...",
        "origin_name": "character.glb",
        "extension": "glb",
        "asset_size": 1234567,
        "parent_id": null,
        "created_at": 1705123456,
        "topic": "my-topic",
        "metadata": {"key1": "value1"}
      }
    ],
    "total": 150,
    "topics_queried": ["topic1"]
  }
  ` + "```" + `

  ## Constraints
  - Query limit: max 10,000 results
  - Metadata key: max 256 characters
  - Metadata value: max 10MB
  - Topics array can be empty (queries all topics)

  ## Error Codes
  | Code | Description |
  |------|-------------|
  | PRESET_NOT_FOUND | Query preset does not exist |
  | MISSING_PARAM | Required parameter not provided |
  | QUERY_ERROR | SQL execution failed |
  | TOPIC_NOT_FOUND | Specified topic does not exist |

  ## Complete Example
  ` + "```" + `python
  import requests

  BASE_URL = "{{base_url}}"

  # First, discover available presets
  presets_response = requests.get(f"{BASE_URL}/api/queries")
  presets = presets_response.json()
  print("Available presets:", [p["name"] for p in presets.get("presets", [])])

  # Option 1: Direct apply
  response = requests.post(
      f"{BASE_URL}/api/metadata/apply",
      json={
          "query_preset": "by-extension",
          "query_params": {"ext": "fbx"},
          "topics": [],  # all topics
          "op": "set",
          "key": "format",
          "value": "fbx",
          "processor": "format-tagger",
          "processor_version": "1.0"
      }
  )
  print(response.json())

  # Option 2: Query first, review, then batch apply
  query_response = requests.post(
      f"{BASE_URL}/api/query/without-metadata",
      json={"topics": [], "params": {"limit": "500"}}
  )
  assets = query_response.json()["results"]

  print(f"Found {len(assets)} assets without metadata")
  # Review or filter assets here...

  operations = [
      {"hash": a["asset_id"], "op": "set", "key": "needs_review", "value": True}
      for a in assets
  ]

  batch_response = requests.post(
      f"{BASE_URL}/api/metadata/batch",
      json={
          "operations": operations,
          "processor": "review-tagger",
          "processor_version": "1.0"
      }
  )
  print(batch_response.json())
  ` + "```" + `

  ## Script Requirements
  Your script should:
  1. First discover available presets via GET /api/queries
  2. Choose the appropriate query preset
  3. Decide between direct apply vs query-then-batch
  4. Handle pagination if results exceed limit
  5. Validate query parameters match preset requirements
  6. Report how many assets were affected
  7. Handle errors gracefully

  ---
  ## YOUR CONTEXT
  [Describe what assets you want to find and what metadata to apply]
`

const defaultUploadFiles = `name: upload-files
description: Upload files to a topic with optional lineage tracking
category: assets
template: |
  # File Upload Script

  ## Purpose
  Upload files to MeshBank topics with optional parent-child lineage tracking.

  ## Base URL
  {{base_url}}

  ## API Endpoint
  POST /api/topics/:topic/assets

  ## Request Format
  - Content-Type: multipart/form-data
  - Form fields:
    - **file**: The file to upload (required)
    - **parent_id**: 64-char hash of parent asset (optional, for versioning)

  ## Response Format (New Upload)
  ` + "```" + `json
  {
    "success": true,
    "hash": "a1b2c3d4e5f6...64-hex-chars",
    "skipped": false,
    "size": 1234567
  }
  ` + "```" + `

  ## Response Format (Duplicate Detected)
  ` + "```" + `json
  {
    "success": true,
    "hash": "a1b2c3d4e5f6...64-hex-chars",
    "skipped": true,
    "existing_topic": "other-topic"
  }
  ` + "```" + `

  ## Constraints
  - Topic name: lowercase alphanumeric, hyphens, underscores only (regex: ^[a-z0-9_-]+$)
  - Topic name length: 1-64 characters
  - File size: limited by server's max_dat_size config (default 1GB)
  - Hash: BLAKE3, returned as 64 hex characters
  - Parent ID must be an existing asset hash (64 hex chars)

  ## Duplicate Handling
  MeshBank uses content-addressed storage:
  - Files are hashed with BLAKE3 before storage
  - If hash already exists anywhere, upload is skipped
  - Response includes existing_topic if found elsewhere
  - Same content = same hash, regardless of filename

  ## Lineage (Parent-Child Relationships)
  Use parent_id to track asset versions:
  - Set parent_id to the hash of the original/previous version
  - Query lineage with "lineage" or "derived" presets
  - Parent must exist in the system before upload

  ## Error Codes
  | Code | Description |
  |------|-------------|
  | TOPIC_NOT_FOUND | Topic does not exist |
  | INVALID_TOPIC_NAME | Topic name doesn't match pattern |
  | ASSET_TOO_LARGE | File exceeds max_dat_size limit |
  | PARENT_NOT_FOUND | Specified parent_id doesn't exist |
  | INVALID_REQUEST | Missing file or malformed request |

  ## Complete Example
  ` + "```" + `python
  import requests
  import os
  from pathlib import Path

  BASE_URL = "{{base_url}}"
  TOPIC = "my-topic"  # Replace with your topic name

  def upload_file(filepath, parent_id=None):
      """Upload a single file, optionally with parent lineage."""
      with open(filepath, 'rb') as f:
          files = {'file': (os.path.basename(filepath), f)}
          data = {}
          if parent_id:
              data['parent_id'] = parent_id

          response = requests.post(
              f"{BASE_URL}/api/topics/{TOPIC}/assets",
              files=files,
              data=data
          )

      result = response.json()
      if result.get('skipped'):
          print(f"Skipped (exists in {result.get('existing_topic', 'same topic')}): {filepath}")
      else:
          print(f"Uploaded: {filepath} -> {result['hash']}")

      return result

  def upload_directory(directory, extensions=None):
      """Upload all files in a directory."""
      results = {"uploaded": 0, "skipped": 0, "failed": 0, "hashes": []}

      for path in Path(directory).rglob('*'):
          if not path.is_file():
              continue
          if extensions and path.suffix.lower() not in extensions:
              continue

          try:
              result = upload_file(str(path))
              if result.get('success'):
                  results["hashes"].append(result['hash'])
                  if result.get('skipped'):
                      results["skipped"] += 1
                  else:
                      results["uploaded"] += 1
              else:
                  results["failed"] += 1
          except Exception as e:
              print(f"Error uploading {path}: {e}")
              results["failed"] += 1

      return results

  def upload_with_lineage(original_path, derived_path):
      """Upload original, then derived with parent link."""
      # Upload original first
      original = upload_file(original_path)
      if not original.get('success'):
          return None

      # Upload derived with parent reference
      derived = upload_file(derived_path, parent_id=original['hash'])
      return {"original": original['hash'], "derived": derived['hash']}

  # Usage examples:
  # Single file
  # upload_file("/path/to/model.glb")

  # Directory with filter
  # results = upload_directory("/path/to/assets", extensions=['.glb', '.fbx'])

  # With lineage
  # upload_with_lineage("/path/to/original.glb", "/path/to/modified.glb")
  ` + "```" + `

  ## Script Requirements
  Your script should:
  1. Handle both single files and directories
  2. Support file extension filtering
  3. Track upload results (uploaded/skipped/failed counts)
  4. Handle lineage if parent_id is needed
  5. Save uploaded hashes for later metadata operations
  6. Handle network errors with retries
  7. Show progress for large batches

  ---
  ## YOUR CONTEXT
  [Describe your files location, any lineage requirements, and target topic]
`

const defaultLineageRelationships = `name: lineage-relationships
description: Query and establish parent-child relationships between assets
category: assets
template: |
  # Lineage Relationships

  ## Purpose
  Manage parent-child relationships between assets for version tracking.

  ## Base URL
  {{base_url}}

  ## Discovering Available Queries
  Before writing your script, discover available query presets:
  GET {{base_url}}/api/queries

  Look for lineage-related presets like: lineage, derived, orphans, roots-with-children

  ## Understanding Lineage
  - **parent_id**: Set during upload to link to an existing asset
  - **Ancestors**: Chain of parents going up (original -> v1 -> v2)
  - **Descendants**: All children and their children going down
  - **Root**: Asset with no parent (original file)
  - **Orphan**: Root with no children

  ## Query Presets for Lineage

  ### Get Ancestor Chain
  POST /api/query/lineage
  ` + "```" + `json
  {
    "topics": [],
    "params": {"hash": "abc123..."}
  }
  ` + "```" + `
  Returns: The asset and all its ancestors up to the root.

  ### Get All Descendants
  POST /api/query/derived
  ` + "```" + `json
  {
    "topics": [],
    "params": {"hash": "abc123..."}
  }
  ` + "```" + `
  Returns: All children and their descendants (recursive).

  ### Find Orphan Assets
  POST /api/query/orphans
  ` + "```" + `json
  {
    "topics": [],
    "params": {"limit": "100"}
  }
  ` + "```" + `
  Returns: Root assets that have no children (potential cleanup candidates).

  ### Find Roots With Children
  POST /api/query/roots-with-children
  ` + "```" + `json
  {
    "topics": [],
    "params": {"limit": "100"}
  }
  ` + "```" + `
  Returns: Original assets that have derived versions, with child count.

  ## Establishing Lineage During Upload
  POST /api/topics/:topic/assets (multipart/form-data)
  - file: The new version
  - parent_id: Hash of the parent asset (must exist)

  ## Query Response Structure
  ` + "```" + `json
  {
    "results": [
      {
        "asset_id": "abc123...",
        "origin_name": "model_v1.glb",
        "extension": "glb",
        "asset_size": 1234567,
        "parent_id": null,
        "blob_name": "000001.dat",
        "created_at": 1705123456,
        "depth": 0,
        "topic": "my-topic"
      },
      {
        "asset_id": "def456...",
        "origin_name": "model_v2.glb",
        "parent_id": "abc123...",
        "depth": 1
      }
    ]
  }
  ` + "```" + `

  ## Constraints
  - parent_id must be exactly 64 hex characters
  - Parent asset must exist before uploading child
  - Lineage is immutable once set (cannot change parent after upload)
  - Depth in lineage query indicates distance from starting asset

  ## Error Codes
  | Code | Description |
  |------|-------------|
  | PARENT_NOT_FOUND | parent_id hash doesn't exist |
  | INVALID_HASH | Hash is not 64 hex characters |
  | ASSET_NOT_FOUND | Starting hash for lineage query not found |

  ## Complete Example
  ` + "```" + `python
  import requests
  import json

  BASE_URL = "{{base_url}}"

  def get_lineage(hash):
      """Get ancestor chain for an asset."""
      response = requests.post(
          f"{BASE_URL}/api/query/lineage",
          json={"topics": [], "params": {"hash": hash}}
      )
      return response.json()["results"]

  def get_descendants(hash):
      """Get all derived versions of an asset."""
      response = requests.post(
          f"{BASE_URL}/api/query/derived",
          json={"topics": [], "params": {"hash": hash}}
      )
      return response.json()["results"]

  def find_roots_needing_review():
      """Find original assets that have been modified."""
      response = requests.post(
          f"{BASE_URL}/api/query/roots-with-children",
          json={"topics": [], "params": {"limit": "500"}}
      )
      return response.json()["results"]

  def upload_version(topic, filepath, parent_hash):
      """Upload a new version linked to parent."""
      with open(filepath, 'rb') as f:
          response = requests.post(
              f"{BASE_URL}/api/topics/{topic}/assets",
              files={'file': f},
              data={'parent_id': parent_hash}
          )
      return response.json()

  def build_lineage_tree(hash):
      """Build a tree structure of all descendants."""
      descendants = get_descendants(hash)
      tree = {"hash": hash, "children": []}

      # Group by parent
      by_parent = {}
      for d in descendants:
          parent = d.get("parent_id")
          if parent not in by_parent:
              by_parent[parent] = []
          by_parent[parent].append(d)

      def add_children(node):
          children = by_parent.get(node["hash"], [])
          for child in children:
              child_node = {"hash": child["asset_id"], "name": child["origin_name"], "children": []}
              node["children"].append(child_node)
              add_children(child_node)

      add_children(tree)
      return tree

  def tag_all_versions(root_hash, key, value):
      """Apply metadata to a root and all its descendants."""
      # Get all descendants
      descendants = get_descendants(root_hash)
      all_hashes = [root_hash] + [d["asset_id"] for d in descendants]

      # Build batch operations
      operations = [
          {"hash": h, "op": "set", "key": key, "value": value}
          for h in all_hashes
      ]

      response = requests.post(
          f"{BASE_URL}/api/metadata/batch",
          json={
              "operations": operations,
              "processor": "lineage-tagger",
              "processor_version": "1.0"
          }
      )
      return response.json()

  # Example usage:
  # ancestors = get_lineage("abc123...")
  # tree = build_lineage_tree("root_hash")
  # tag_all_versions("root_hash", "family", "character-001")
  ` + "```" + `

  ## Script Requirements
  Your script should:
  1. Query lineage chains to understand asset relationships
  2. Upload new versions with correct parent_id references
  3. Handle missing parents gracefully
  4. Build tree visualizations if needed
  5. Support tagging entire lineage trees with metadata

  ---
  ## YOUR CONTEXT
  [Describe your lineage needs: querying existing relationships, establishing new ones, or both]
`

const defaultConditionalMetadata = `name: conditional-metadata
description: Apply metadata based on conditions like extension, size, or existing metadata
category: metadata
template: |
  # Conditional Metadata Operations

  ## Purpose
  Apply metadata to assets based on conditions: file properties, existing metadata, or query results.

  ## Base URL
  {{base_url}}

  ## Discovering Available Queries
  Before writing your script, discover available query presets:
  GET {{base_url}}/api/queries

  This returns all presets you can use for filtering, including queries for:
  - File properties (by-extension, large-files, recent-imports, by-origin-name)
  - Metadata state (without-metadata, with-metadata, by-processor)
  - Lineage (lineage, derived, orphans, roots-with-children)

  ## Strategy Overview
  1. Query assets using appropriate preset
  2. Filter results based on your conditions
  3. Apply metadata to filtered set via batch endpoint

  ## Query Response Fields for Filtering
  ` + "```" + `json
  {
    "asset_id": "abc123...",
    "origin_name": "character_rig_v2.glb",
    "extension": "glb",
    "asset_size": 15234567,
    "parent_id": "def456...",
    "created_at": 1705123456,
    "topic": "characters",
    "metadata": {
      "existing_key": "existing_value"
    }
  }
  ` + "```" + `

  ## Batch Metadata Endpoint
  POST /api/metadata/batch
  ` + "```" + `json
  {
    "operations": [
      {"hash": "...", "op": "set", "key": "...", "value": ...}
    ],
    "processor": "conditional-tagger",
    "processor_version": "1.0"
  }
  ` + "```" + `

  ## Constraints
  - Query limit: max 10,000 results per query
  - Batch limit: max 100,000 operations
  - Metadata key: max 256 characters
  - Metadata value: max 10MB

  ## Complete Example
  ` + "```" + `python
  import requests

  BASE_URL = "{{base_url}}"

  # First, discover available presets
  presets_response = requests.get(f"{BASE_URL}/api/queries")
  presets = presets_response.json()
  print("Available presets:", [p["name"] for p in presets.get("presets", [])])

  def query_assets(preset, params, topics=None):
      """Query assets with given preset."""
      response = requests.post(
          f"{BASE_URL}/api/query/{preset}",
          json={"topics": topics or [], "params": params}
      )
      return response.json().get("results", [])

  def apply_metadata(assets, key, value, processor="conditional-script"):
      """Apply metadata to list of assets."""
      if not assets:
          return {"success": True, "total": 0}

      operations = [
          {"hash": a["asset_id"], "op": "set", "key": key, "value": value}
          for a in assets
      ]

      response = requests.post(
          f"{BASE_URL}/api/metadata/batch",
          json={
              "operations": operations,
              "processor": processor,
              "processor_version": "1.0"
          }
      )
      return response.json()

  # ============================================================================
  # CONDITION: By Extension
  # ============================================================================
  def tag_by_extension(extension, key, value):
      """Tag all files with specific extension."""
      assets = query_assets("by-extension", {"ext": extension, "limit": "10000"})
      return apply_metadata(assets, key, value)

  # Example: Tag all .glb files as 3D models
  # tag_by_extension("glb", "type", "3d-model")

  # ============================================================================
  # CONDITION: By Size
  # ============================================================================
  def tag_large_files(min_bytes, key, value):
      """Tag files larger than threshold."""
      assets = query_assets("large-files", {"min_size": str(min_bytes), "limit": "10000"})
      return apply_metadata(assets, key, value)

  # Example: Mark files over 100MB as "needs-optimization"
  # tag_large_files(100_000_000, "needs_optimization", True)

  # ============================================================================
  # CONDITION: By Existing Metadata
  # ============================================================================
  def tag_if_missing_key(target_key, value):
      """Tag assets that don't have computed metadata."""
      assets = query_assets("without-metadata", {"limit": "10000"})
      return apply_metadata(assets, target_key, value)

  def tag_if_has_metadata(existing_key, new_key, new_value):
      """Tag assets that have a specific metadata key."""
      assets = query_assets("with-metadata", {"key": existing_key, "limit": "10000"})
      return apply_metadata(assets, new_key, new_value)

  # Example: Mark unprocessed assets for review
  # tag_if_missing_key("needs_review", True)

  # ============================================================================
  # CONDITION: By Filename Pattern
  # ============================================================================
  def tag_by_name_pattern(pattern, key, value):
      """Tag assets whose filename contains pattern."""
      assets = query_assets("by-origin-name", {"name": pattern, "limit": "10000"})
      return apply_metadata(assets, key, value)

  # Example: Tag all "character_" files
  # tag_by_name_pattern("character_", "category", "character")

  # ============================================================================
  # CONDITION: By Lineage (has parent or is root)
  # ============================================================================
  def tag_versioned_assets(key, value):
      """Tag assets that have a parent (are derived versions)."""
      # Query recent assets and filter by parent_id
      assets = query_assets("recent-imports", {"days": "365", "limit": "10000"})
      versioned = [a for a in assets if a.get("parent_id")]
      return apply_metadata(versioned, key, value)

  def tag_root_assets(key, value):
      """Tag assets with no parent (original versions)."""
      assets = query_assets("recent-imports", {"days": "365", "limit": "10000"})
      roots = [a for a in assets if not a.get("parent_id")]
      return apply_metadata(roots, key, value)

  # ============================================================================
  # CONDITION: Combined/Complex
  # ============================================================================
  def tag_large_glb_without_metadata():
      """Complex: large GLB files that haven't been processed."""
      # Get large files
      large = query_assets("large-files", {"min_size": "50000000", "limit": "10000"})

      # Filter to GLB extension
      large_glb = [a for a in large if a.get("extension") == "glb"]

      # Filter to those without metadata
      unprocessed = [a for a in large_glb if not a.get("metadata")]

      print(f"Found {len(unprocessed)} large GLB files without metadata")
      return apply_metadata(unprocessed, "priority", "high")

  def tag_by_metadata_value(search_key, search_value, new_key, new_value):
      """Tag assets where existing metadata matches a value."""
      assets = query_assets("with-metadata", {"key": search_key, "limit": "10000"})

      # Filter by value
      matching = [
          a for a in assets
          if a.get("metadata", {}).get(search_key) == search_value
      ]

      return apply_metadata(matching, new_key, new_value)

  # Example: Tag approved assets as "ready-for-production"
  # tag_by_metadata_value("status", "approved", "stage", "production")
  ` + "```" + `

  ## Script Requirements
  Your script should:
  1. First discover available presets via GET /api/queries
  2. Choose appropriate query preset for initial filtering
  3. Apply additional filters on query results
  4. Handle pagination if results exceed 10,000
  5. Validate conditions before applying metadata
  6. Report how many assets matched each condition
  7. Support dry-run mode to preview changes
  8. Handle errors and partial failures

  ---
  ## YOUR CONTEXT
  [Describe your conditions and what metadata to apply]
`
