import type { Carousel } from "./carousel";

export class SwipeGestureHandler {
  private startX = 0;
  private startY = 0;
  private currentX = 0;
  private startTime = 0;
  private isDragging = false;
  private element: HTMLElement;
  private carousel: Carousel;

  // Thresholds
  private readonly SWIPE_THRESHOLD = 80; // px
  private readonly VELOCITY_THRESHOLD = 0.5; // px/ms
  private readonly EDGE_RESISTANCE = 0.3; // Dampening factor for edge bounce
  private readonly MAX_EDGE_DISTANCE = 50; // Maximum pixels to drag at edge

  constructor(element: HTMLElement, carousel: Carousel) {
    this.element = element;
    this.carousel = carousel;

    // Bind touch events
    this.element.addEventListener(
      "touchstart",
      this.handleTouchStart.bind(this),
      {
        passive: false,
      },
    );
    this.element.addEventListener(
      "touchmove",
      this.handleTouchMove.bind(this),
      {
        passive: false,
      },
    );
    this.element.addEventListener("touchend", this.handleTouchEnd.bind(this));
    this.element.addEventListener(
      "touchcancel",
      this.handleTouchEnd.bind(this),
    );

    // Also support mouse events for desktop drag
    this.element.addEventListener("mousedown", this.handleMouseDown.bind(this));
    this.element.addEventListener("mousemove", this.handleMouseMove.bind(this));
    this.element.addEventListener("mouseup", this.handleMouseUp.bind(this));
    this.element.addEventListener("mouseleave", this.handleMouseUp.bind(this));
  }

  private handleTouchStart(e: TouchEvent): void {
    this.startX = e.touches[0].clientX;
    this.startY = e.touches[0].clientY;
    this.currentX = this.startX;
    this.startTime = Date.now();
    this.isDragging = true;
  }

  private handleTouchMove(e: TouchEvent): void {
    if (!this.isDragging) return;

    this.currentX = e.touches[0].clientX;
    const currentY = e.touches[0].clientY;

    const deltaX = this.currentX - this.startX;
    const deltaY = currentY - this.startY;

    // Detect if this is primarily a horizontal swipe
    if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > 10) {
      // Prevent vertical scroll during horizontal swipe
      e.preventDefault();

      // Apply live drag feedback with edge resistance
      this.applyDragTransform(deltaX);
    }
  }

  private handleTouchEnd(_e: TouchEvent): void {
    if (!this.isDragging) return;

    const deltaX = this.currentX - this.startX;
    const deltaTime = Date.now() - this.startTime;
    const velocity = Math.abs(deltaX) / Math.max(deltaTime, 1);

    // Clear any drag transform
    this.clearDragTransform();

    // Determine if we should navigate
    const shouldSwipeLeft =
      deltaX < -this.SWIPE_THRESHOLD ||
      (velocity > this.VELOCITY_THRESHOLD && deltaX < 0);
    const shouldSwipeRight =
      deltaX > this.SWIPE_THRESHOLD ||
      (velocity > this.VELOCITY_THRESHOLD && deltaX > 0);

    if (shouldSwipeLeft) {
      this.carousel.next();
    } else if (shouldSwipeRight) {
      this.carousel.previous();
    }

    this.isDragging = false;
  }

  private handleMouseDown(e: MouseEvent): void {
    // Only handle left mouse button
    if (e.button !== 0) return;

    this.startX = e.clientX;
    this.startY = e.clientY;
    this.currentX = this.startX;
    this.startTime = Date.now();
    this.isDragging = true;

    e.preventDefault();
  }

  private handleMouseMove(e: MouseEvent): void {
    if (!this.isDragging) return;

    this.currentX = e.clientX;
    const currentY = e.clientY;

    const deltaX = this.currentX - this.startX;
    const deltaY = currentY - this.startY;

    // Detect if this is primarily a horizontal swipe
    if (Math.abs(deltaX) > Math.abs(deltaY) && Math.abs(deltaX) > 10) {
      e.preventDefault();
      this.applyDragTransform(deltaX);
    }
  }

  private handleMouseUp(_e: MouseEvent): void {
    if (!this.isDragging) return;

    const deltaX = this.currentX - this.startX;
    const deltaTime = Date.now() - this.startTime;
    const velocity = Math.abs(deltaX) / Math.max(deltaTime, 1);

    // Clear any drag transform
    this.clearDragTransform();

    // Determine if we should navigate
    const shouldSwipeLeft =
      deltaX < -this.SWIPE_THRESHOLD ||
      (velocity > this.VELOCITY_THRESHOLD && deltaX < 0);
    const shouldSwipeRight =
      deltaX > this.SWIPE_THRESHOLD ||
      (velocity > this.VELOCITY_THRESHOLD && deltaX > 0);

    if (shouldSwipeLeft) {
      this.carousel.next();
    } else if (shouldSwipeRight) {
      this.carousel.previous();
    }

    this.isDragging = false;
  }

  /**
   * Apply a transform to the current slide during drag with edge resistance
   */
  private applyDragTransform(deltaX: number): void {
    const isAtStart = this.carousel.isAtStart();
    const isAtEnd = this.carousel.isAtEnd();

    // Calculate transform with edge resistance
    let transform = deltaX;

    if (isAtStart && deltaX > 0) {
      // At first camera, swiping right - apply resistance
      transform = Math.min(
        deltaX * this.EDGE_RESISTANCE,
        this.MAX_EDGE_DISTANCE,
      );
    } else if (isAtEnd && deltaX < 0) {
      // At last camera, swiping left - apply resistance
      transform = Math.max(
        deltaX * this.EDGE_RESISTANCE,
        -this.MAX_EDGE_DISTANCE,
      );
    }

    // Apply transform to carousel container
    this.element.style.transform = `translateX(${transform}px)`;
    this.element.style.transition = "none";
  }

  /**
   * Clear the drag transform and reset to default state
   */
  private clearDragTransform(): void {
    this.element.style.transform = "";
    this.element.style.transition = "";
  }

  /**
   * Clean up event listeners
   */
  destroy(): void {
    this.element.removeEventListener(
      "touchstart",
      this.handleTouchStart.bind(this),
    );
    this.element.removeEventListener(
      "touchmove",
      this.handleTouchMove.bind(this),
    );
    this.element.removeEventListener(
      "touchend",
      this.handleTouchEnd.bind(this),
    );
    this.element.removeEventListener(
      "touchcancel",
      this.handleTouchEnd.bind(this),
    );

    this.element.removeEventListener(
      "mousedown",
      this.handleMouseDown.bind(this),
    );
    this.element.removeEventListener(
      "mousemove",
      this.handleMouseMove.bind(this),
    );
    this.element.removeEventListener("mouseup", this.handleMouseUp.bind(this));
    this.element.removeEventListener(
      "mouseleave",
      this.handleMouseUp.bind(this),
    );
  }
}
