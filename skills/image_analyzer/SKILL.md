---
name: image-analyzer
description: Analyze image technical details: dimensions, format, EXIF metadata, file size
activation_keywords: [image, photo, picture, dimensions, resolution, exif, metadata, format]
execution_mode: client
---

# Image Analyzer Skill

Provides technical image analysis:
- Get image dimensions (width x height)
- Detect format (JPEG, PNG, GIF, WebP, BMP)
- Extract EXIF metadata (camera, GPS, timestamp)
- Calculate file size and aspect ratio
- Check image quality indicators

Use `builtin_image_analyzer` tool with fields:
- `operation`: one of "info", "exif", "dimensions", "format"
- `file_path`: path to the image file (required)

Note: This is technical metadata analysis, NOT visual content recognition.
For "what's in this image" questions, use the model's built-in vision capability.
