import type { Detection } from "@mediapipe/tasks-vision";
import { FilesetResolver } from "@mediapipe/tasks-vision";

type Point = { d: Detection; x: number; y: number };

class DetectionsSet extends Set<Detection> {
  private maxLength: number;
  private points: Point[] = [];
  private videoElement: HTMLVideoElement;
  private parent: HTMLElement;
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private colorRGB: string; // "R,G,B"
  private minAlpha: number;
  private resizeBound: () => void;

  constructor(
    maxLength: number,
    videoElement: HTMLVideoElement,
    colorRGB: `${number},${number},${number}`, // e.g. "0,200,255"
    minAlpha = 0.15,
    iterable: readonly Detection[] | null = null,
  ) {
    super(iterable ?? []);
    this.maxLength = maxLength;
    this.videoElement = videoElement;
    this.parent = videoElement.parentElement!;
    this.colorRGB = colorRGB;
    this.minAlpha = minAlpha;

    // create canvas overlay
    this.canvas = document.createElement("canvas");
    this.canvas.style.position = "absolute";
    this.canvas.style.pointerEvents = "none";
    this.canvas.style.zIndex = "999"; // adjust if needed
    this.parent.appendChild(this.canvas);

    const ctx = this.canvas.getContext("2d");
    if (!ctx) throw new Error("Cannot get 2D context");
    this.ctx = ctx;

    // sync size & position initially and on resize
    this.resizeBound = this.syncCanvas.bind(this);
    window.addEventListener("resize", this.resizeBound);
    // also try to catch video metadata load in case the video sizes change
    this.videoElement.addEventListener("loadedmetadata", this.resizeBound);
    this.syncCanvas();
  }

  // keep canvas size/position in sync with the video element
  private syncCanvas() {
    const cssW = this.videoElement.scrollWidth || this.videoElement.clientWidth;
    const cssH =
      this.videoElement.scrollHeight || this.videoElement.clientHeight;
    const dpr = window.devicePixelRatio || 1;

    // position the canvas on top of the video
    const rect = this.videoElement.getBoundingClientRect();
    // parent coordinates: position canvas relative to parent element
    const parentRect = this.parent.getBoundingClientRect();
    const top = rect.top - parentRect.top;
    const left = rect.left - parentRect.left;

    this.canvas.style.top = `${top}px`;
    this.canvas.style.left = `${left}px`;
    this.canvas.style.width = `${cssW}px`;
    this.canvas.style.height = `${cssH}px`;

    // set internal pixel resolution for crisp lines on high-DPI displays
    this.canvas.width = Math.max(1, Math.round(cssW * dpr));
    this.canvas.height = Math.max(1, Math.round(cssH * dpr));

    // scale drawing so coordinates are in CSS pixels
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

    // redraw after resize
    this.drawPath();
  }

  // remove a detection (and its point) and redraw
  delete(value: Detection): boolean {
    const idx = this.points.findIndex((p) => p.d === value);
    if (idx >= 0) this.points.splice(idx, 1);
    const res = super.delete(value);
    this.drawPath();
    return res;
  }

  // compute the bottom-center 10% up point (relative to video coordinates)
  private computePointForDetection(
    d: Detection,
  ): { x: number; y: number } | null {
    const b = d.boundingBox;
    if (!b) return null;

    // scale from video coordinate space to displayed CSS pixels
    const vidW = this.videoElement.videoWidth || 1;
    const vidH = this.videoElement.videoHeight || 1;
    const xScale = this.videoElement.scrollWidth / vidW;
    const yScale = this.videoElement.scrollHeight / vidH;

    const cx = (b.originX + b.width / 2) * xScale;
    const cy = (b.originY + b.height * 0.9) * yScale; // 10% up from bottom

    return { x: cx, y: cy };
  }

  // add detection, maintain size limit, and redraw
  add(detection: Detection) {
    if (!detection.boundingBox) return this;

    // if already present, update its point instead of re-adding
    if (this.has(detection)) {
      const idx = this.points.findIndex((p) => p.d === detection);
      if (idx >= 0) {
        const pt = this.computePointForDetection(detection);
        if (pt) {
          this.points[idx].x = pt.x;
          this.points[idx].y = pt.y;
          this.drawPath();
        }
      }
      return this;
    }

    // if adding would exceed limit, delete first (oldest) element
    if (this.size >= this.maxLength) {
      const first = this.values().next().value;
      if (first) this.delete(first);
    }

    const pt = this.computePointForDetection(detection);
    if (!pt) return this;

    this.points.push({ d: detection, x: pt.x, y: pt.y });
    super.add(detection);

    this.drawPath();
    return this;
  }

  // draw a smooth path by stroking each segment individually with increasing alpha
  private drawPath() {
    const pts = this.points.map((p) => ({ x: p.x, y: p.y }));
    const n = pts.length;

    this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    if (n < 2) return;

    this.ctx.lineWidth = 2;
    this.ctx.lineJoin = "round";
    this.ctx.lineCap = "round";

    const tension = 1.0;

    // loop over each segment (P1..P2) with P0 and P3 as neighbors
    for (let i = 0; i < n - 1; i++) {
      const p0 = pts[i - 1] ?? pts[i]; // clamp at edges
      const p1 = pts[i];
      const p2 = pts[i + 1];
      const p3 = pts[i + 2] ?? p2;

      // Catmull–Rom to cubic Bézier
      const cp1x = p1.x + ((p2.x - p0.x) / 6) * tension;
      const cp1y = p1.y + ((p2.y - p0.y) / 6) * tension;
      const cp2x = p2.x - ((p3.x - p1.x) / 6) * tension;
      const cp2y = p2.y - ((p3.y - p1.y) / 6) * tension;

      const t = i / (n - 1);
      const alpha = this.minAlpha + t * (1 - this.minAlpha);

      this.ctx.beginPath();
      this.ctx.moveTo(p1.x, p1.y);
      this.ctx.bezierCurveTo(cp1x, cp1y, cp2x, cp2y, p2.x, p2.y);
      this.ctx.strokeStyle = `rgba(${this.colorRGB}, ${alpha})`;
      this.ctx.stroke();
    }
  }

  // remove canvas and listeners if you want to destroy this instance
  destroy() {
    window.removeEventListener("resize", this.resizeBound);
    this.videoElement.removeEventListener("loadedmetadata", this.resizeBound);
    this.canvas.remove();
    this.points = [];
    // clear Set items
    super.clear();
  }
}

export const startObjectDetection = async (videoElement: HTMLVideoElement) => {
  const detectionLists = {
    dog: new DetectionsSet(100, videoElement, "255,0,0"),
    person: new DetectionsSet(100, videoElement, "0,0,255"),
  };
  const { ObjectDetector } = await import("@mediapipe/tasks-vision");
  const vision = await FilesetResolver.forVisionTasks("./wasm");
  const objectDetector = await ObjectDetector.createFromOptions(vision, {
    baseOptions: {
      modelAssetPath: `./models/efficientdet_lite0.tflite`,
    },
    scoreThreshold: 0.5,
    runningMode: "VIDEO",
    categoryAllowlist: ["person", "dog"],
  });
  let lastVideoTime = -1;
  let lastDetectionTime = 0; // ms timestamp of last detection
  const DETECTION_INTERVAL = 100; // ms
  function renderLoop(): void {
    if (videoElement.currentTime !== lastVideoTime) {
      const now = performance.now();
      if (now - lastDetectionTime >= DETECTION_INTERVAL) {
        const { detections } = objectDetector.detectForVideo(videoElement, now);
        for (const detection of detections) {
          const category = detection.categories[0].categoryName;
          if (category in detectionLists) {
            detectionLists[category as keyof typeof detectionLists].add(
              detection,
            );
          }
        }
        lastDetectionTime = now;
        lastVideoTime = videoElement.currentTime;
      }
    }
    requestAnimationFrame(() => {
      renderLoop();
    });
  }
  renderLoop();
};
