---
name: soppy-excalidraw
description: Draw and colour Excalidraw diagrams for the VS Code extension pomdtr.excalidraw-editor. Use when creating, editing, or reviewing a .excalidraw or .excalidraw.svg file, or when a diagram is needed in a document.
---

# Excalidraw diagrams

Diagrams here are files committed beside the document that uses them, edited in the
`pomdtr.excalidraw-editor` VS Code extension.

## Draw for the reader

- One idea per diagram. Two diagrams that each say one thing beat one that says both.
- Lay it out left to right or top to bottom, and keep the flow one-directional. A reader
  who has to trace a line backwards has lost the thread.
- Label every arrow. An unlabelled arrow is a guess about who is asking.
- Keep the vocabulary consistent with the document: if the prose says "client", the box
  says "client" too.

## Colour

- Colour carries meaning or it is removed. Pick a small set and write down what each one
  means — usually one colour for the thing being discussed, and a neutral for everything
  else.
- Use Excalidraw's own palette so the diagram survives a theme change.
- Do not rely on colour alone to distinguish two things; a red and a green box are the
  same box to a reader who cannot tell them apart.
- Fill, then border, then text. A diagram that reads in greyscale is finished.

## Files

- Commit the `.excalidraw` source next to the exported `.svg`. The export is what the
  document shows; the source is what somebody can change.
- Re-export after editing. A stale export is the version everyone reads.
