#import <AppKit/AppKit.h>

static void svvyPositionTrafficLightsOnMainThread(void *windowPointer, CGFloat leading, CGFloat top) {
  if (!windowPointer) {
    return;
  }

  NSWindow *window = (__bridge NSWindow *)windowPointer;
  NSButton *closeButton = [window standardWindowButton:NSWindowCloseButton];
  NSButton *minimizeButton = [window standardWindowButton:NSWindowMiniaturizeButton];
  NSButton *zoomButton = [window standardWindowButton:NSWindowZoomButton];

  if (!closeButton || !minimizeButton || !zoomButton || !closeButton.superview) {
    return;
  }

  NSView *titlebarView = closeButton.superview;
  NSRect closeFrame = closeButton.frame;
  NSRect minimizeFrame = minimizeButton.frame;
  NSRect zoomFrame = zoomButton.frame;
  CGFloat gap = minimizeFrame.origin.x - closeFrame.origin.x;

  if (gap <= 0) {
    gap = zoomFrame.origin.x - minimizeFrame.origin.x;
  }

  if (gap <= 0) {
    gap = closeFrame.size.width + 6.0;
  }

  CGFloat y = NSHeight(titlebarView.bounds) - top - NSHeight(closeFrame);
  [closeButton setFrameOrigin:NSMakePoint(leading, y)];
  [minimizeButton setFrameOrigin:NSMakePoint(leading + gap, y)];
  [zoomButton setFrameOrigin:NSMakePoint(leading + gap * 2.0, y)];
}

void svvyPositionTrafficLights(void *windowPointer, double leading, double top) {
  if ([NSThread isMainThread]) {
    svvyPositionTrafficLightsOnMainThread(windowPointer, (CGFloat)leading, (CGFloat)top);
    return;
  }

  dispatch_sync(dispatch_get_main_queue(), ^{
    svvyPositionTrafficLightsOnMainThread(windowPointer, (CGFloat)leading, (CGFloat)top);
  });
}
