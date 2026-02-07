import type { Carousel } from "./carousel";

export class NavigationUI {
  private carousel: Carousel;
  private indicatorContainer: HTMLElement;
  private arrowPrev: HTMLElement | null = null;
  private arrowNext: HTMLElement | null = null;
  private dots: HTMLElement[] = [];

  constructor(carousel: Carousel, navContainer: HTMLElement) {
    this.carousel = carousel;
    this.indicatorContainer = navContainer;

    this.createIndicatorDots();
    this.createArrowButtons();
    this.setupKeyboardNavigation();

    // Listen for carousel index changes to update UI
    this.carousel.onIndexChange(this.updateIndicators.bind(this));
  }

  /**
   * Create indicator dots for camera navigation
   */
  private createIndicatorDots(): void {
    const cameraCount = this.carousel.getCameraCount();

    // Only show dots if there are multiple cameras
    if (cameraCount <= 1) {
      return;
    }

    const dotsWrapper = document.createElement("div");
    dotsWrapper.className = "carousel-indicators";
    dotsWrapper.setAttribute("role", "tablist");
    dotsWrapper.setAttribute("aria-label", "Camera selection");

    for (let i = 0; i < cameraCount; i++) {
      const dot = document.createElement("button");
      dot.className = "indicator-dot";
      dot.type = "button";
      dot.setAttribute("role", "tab");
      dot.setAttribute("aria-label", this.carousel.getCamera(i).title);
      dot.setAttribute(
        "aria-selected",
        i === this.carousel.getCurrentIndex() ? "true" : "false",
      );

      if (i === this.carousel.getCurrentIndex()) {
        dot.classList.add("active");
      }

      // Click handler
      dot.addEventListener("click", () => {
        this.carousel.jumpTo(i);
      });

      this.dots.push(dot);
      dotsWrapper.appendChild(dot);
    }

    this.indicatorContainer.appendChild(dotsWrapper);
  }

  /**
   * Create previous and next arrow buttons
   */
  private createArrowButtons(): void {
    const cameraCount = this.carousel.getCameraCount();

    // Only show arrows if there are multiple cameras
    if (cameraCount <= 1) {
      return;
    }

    // Previous arrow
    const prevButton = document.createElement("button");
    prevButton.className = "carousel-arrow carousel-arrow-prev";
    prevButton.type = "button";
    prevButton.setAttribute("aria-label", "Previous camera");
    prevButton.innerHTML = `
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="15 18 9 12 15 6"></polyline>
      </svg>
    `;
    prevButton.addEventListener("click", () => {
      this.carousel.previous();
    });
    this.arrowPrev = prevButton;

    // Next arrow
    const nextButton = document.createElement("button");
    nextButton.className = "carousel-arrow carousel-arrow-next";
    nextButton.type = "button";
    nextButton.setAttribute("aria-label", "Next camera");
    nextButton.innerHTML = `
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="9 18 15 12 9 6"></polyline>
      </svg>
    `;
    nextButton.addEventListener("click", () => {
      this.carousel.next();
    });
    this.arrowNext = nextButton;

    // Add to DOM
    const carouselContainer = this.indicatorContainer.parentElement;
    if (carouselContainer) {
      carouselContainer.appendChild(this.arrowPrev);
      carouselContainer.appendChild(this.arrowNext);
    }

    // Update arrow visibility
    this.updateArrowStates();
  }

  /**
   * Setup keyboard navigation (arrow keys and number keys)
   */
  private setupKeyboardNavigation(): void {
    document.addEventListener("keydown", (e: KeyboardEvent) => {
      // Ignore if user is typing in an input
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement
      ) {
        return;
      }

      switch (e.key) {
        case "ArrowLeft":
          e.preventDefault();
          this.carousel.previous();
          break;
        case "ArrowRight":
          e.preventDefault();
          this.carousel.next();
          break;
        case "Home":
          e.preventDefault();
          this.carousel.jumpTo(0);
          break;
        case "End":
          e.preventDefault();
          this.carousel.jumpTo(this.carousel.getCameraCount() - 1);
          break;
        default:
          // Number keys 1-9 to jump to specific camera
          if (e.key >= "1" && e.key <= "9") {
            const cameraIndex = Number.parseInt(e.key, 10) - 1;
            if (cameraIndex < this.carousel.getCameraCount()) {
              e.preventDefault();
              this.carousel.jumpTo(cameraIndex);
            }
          }
          break;
      }
    });
  }

  /**
   * Update indicator dots to reflect current camera
   */
  private updateIndicators(currentIndex: number): void {
    // Update dots
    for (let i = 0; i < this.dots.length; i++) {
      const dot = this.dots[i];
      if (i === currentIndex) {
        dot.classList.add("active");
        dot.setAttribute("aria-selected", "true");
      } else {
        dot.classList.remove("active");
        dot.setAttribute("aria-selected", "false");
      }
    }

    // Update arrow states
    this.updateArrowStates();

    // Announce to screen readers
    this.announceChange(currentIndex);
  }

  /**
   * Update arrow button states based on current position
   */
  private updateArrowStates(): void {
    const isAtStart = this.carousel.isAtStart();
    const isAtEnd = this.carousel.isAtEnd();

    if (this.arrowPrev) {
      if (isAtStart) {
        this.arrowPrev.classList.add("disabled");
        this.arrowPrev.setAttribute("aria-disabled", "true");
      } else {
        this.arrowPrev.classList.remove("disabled");
        this.arrowPrev.setAttribute("aria-disabled", "false");
      }
    }

    if (this.arrowNext) {
      if (isAtEnd) {
        this.arrowNext.classList.add("disabled");
        this.arrowNext.setAttribute("aria-disabled", "true");
      } else {
        this.arrowNext.classList.remove("disabled");
        this.arrowNext.setAttribute("aria-disabled", "false");
      }
    }
  }

  /**
   * Announce camera change to screen readers
   */
  private announceChange(currentIndex: number): void {
    // Create or get the live region for announcements
    let liveRegion = document.getElementById("carousel-live-region");
    if (!liveRegion) {
      liveRegion = document.createElement("div");
      liveRegion.id = "carousel-live-region";
      liveRegion.setAttribute("role", "status");
      liveRegion.setAttribute("aria-live", "polite");
      liveRegion.setAttribute("aria-atomic", "true");
      liveRegion.className = "sr-only";
      document.body.appendChild(liveRegion);
    }

    // Announce the change
    liveRegion.textContent = `Showing ${this.carousel.getCamera(currentIndex).title} (${currentIndex + 1} of ${this.carousel.getCameraCount()})`;
  }
}
