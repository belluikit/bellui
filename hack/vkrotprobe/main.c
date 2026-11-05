// Minimal Vulkan NativeActivity: clears and presents a changing colour,
// recreating the swapchain on resize/config change/out-of-date (#909 control).
#include <android_native_app_glue.h>
#include <android/log.h>
#define VK_USE_PLATFORM_ANDROID_KHR
#include <vulkan/vulkan.h>
#include <stdlib.h>
#include <string.h>
#include <pthread.h>
#ifdef WAIT_REDRAW
// Like bellui: the activity's main thread waits in onNativeWindowRedrawNeeded
// until the render thread has presented a frame after the event.
static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER; static pthread_cond_t cv = PTHREAD_COND_INITIALIZER;
static long presents;
static void redrawNeeded(ANativeActivity *a, ANativeWindow *w) {
  pthread_mutex_lock(&mu); long want = presents + 1;
  __android_log_print(ANDROID_LOG_INFO, "vkrot", "redrawNeeded: waiting for present %ld", want);
  while (presents < want) pthread_cond_wait(&cv, &mu);
  pthread_mutex_unlock(&mu);
  __android_log_print(ANDROID_LOG_INFO, "vkrot", "redrawNeeded: done");
}
#endif

#define LOG(...) __android_log_print(ANDROID_LOG_INFO, "vkrot", __VA_ARGS__)
#define CK(x) do { VkResult r_ = (x); if (r_ != VK_SUCCESS) { LOG("%s = %d", #x, r_); } } while (0)

static VkInstance inst; static VkPhysicalDevice pd; static VkDevice dev; static VkQueue q;
static uint32_t qfam; static VkSurfaceKHR surf; static VkSwapchainKHR sc;
static VkImage imgs[8]; static uint32_t nimg; static VkCommandPool pool;
static VkSemaphore acq, done; static VkFence fence; static int ready, recreates, frames;
static VkFormat fmt; static struct android_app *gapp;
#ifdef WGPU_SYNC
// Like wgpu-hal's Android swapchain: images are acquired with a fence, and every
// reconfigure destroys the old swapchain's fence and semaphores after a
// vkDeviceWaitIdle and makes new ones.
static VkFence afence; static int syncDrops;
static void mkSync(void) {
  VkSemaphoreCreateInfo sci = {VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO};
  vkCreateSemaphore(dev, &sci, 0, &acq); vkCreateSemaphore(dev, &sci, 0, &done);
  VkFenceCreateInfo fci = {VK_STRUCTURE_TYPE_FENCE_CREATE_INFO};
  vkCreateFence(dev, &fci, 0, &afence);
}
static void dropSync(void) {
  vkDestroyFence(dev, afence, 0); vkDestroySemaphore(dev, acq, 0); vkDestroySemaphore(dev, done, 0);
  syncDrops++;
}
#endif

static void initDevice(void) {
  const char *iext[] = {"VK_KHR_surface", "VK_KHR_android_surface"};
  VkApplicationInfo ai = {VK_STRUCTURE_TYPE_APPLICATION_INFO, .apiVersion = VK_API_VERSION_1_1};
  VkInstanceCreateInfo ici = {VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO, .pApplicationInfo = &ai, .enabledExtensionCount = 2, .ppEnabledExtensionNames = iext};
  CK(vkCreateInstance(&ici, 0, &inst));
  uint32_t n = 1; vkEnumeratePhysicalDevices(inst, &n, &pd);
  qfam = 0; float pr = 1;
  VkDeviceQueueCreateInfo qci = {VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO, .queueFamilyIndex = qfam, .queueCount = 1, .pQueuePriorities = &pr};
  const char *dext[] = {"VK_KHR_swapchain"};
  VkDeviceCreateInfo dci = {VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO, .queueCreateInfoCount = 1, .pQueueCreateInfos = &qci, .enabledExtensionCount = 1, .ppEnabledExtensionNames = dext};
  CK(vkCreateDevice(pd, &dci, 0, &dev));
  vkGetDeviceQueue(dev, qfam, 0, &q);
  VkCommandPoolCreateInfo pci = {VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO, .flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT, .queueFamilyIndex = qfam};
  CK(vkCreateCommandPool(dev, &pci, 0, &pool));
  VkSemaphoreCreateInfo sci = {VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO};
  vkCreateSemaphore(dev, &sci, 0, &acq); vkCreateSemaphore(dev, &sci, 0, &done);
  VkFenceCreateInfo fci = {VK_STRUCTURE_TYPE_FENCE_CREATE_INFO, .flags = VK_FENCE_CREATE_SIGNALED_BIT};
  vkCreateFence(dev, &fci, 0, &fence);
}

static void makeSwapchain(const char *why) {
  vkDeviceWaitIdle(dev);
  VkSurfaceCapabilitiesKHR caps; CK(vkGetPhysicalDeviceSurfaceCapabilitiesKHR(pd, surf, &caps));
  uint32_t nf = 1; VkSurfaceFormatKHR sf; vkGetPhysicalDeviceSurfaceFormatsKHR(pd, surf, &nf, &sf); fmt = sf.format;
  VkSwapchainKHR old = sc;
#ifdef WGPU_SYNC
  if (old) dropSync();
  if (old || !afence) mkSync();
#endif
  VkSwapchainCreateInfoKHR ci = {VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR, .surface = surf,
    .minImageCount = caps.minImageCount < 3 ? 3 : caps.minImageCount, .imageFormat = sf.format, .imageColorSpace = sf.colorSpace,
#ifdef WINDOW_EXTENT
    // Like wgpu/bellui: the size bellui configures is the window's current
    // width x height, which swaps with rotation.
    .imageExtent = {(uint32_t)ANativeWindow_getWidth(gapp->window), (uint32_t)ANativeWindow_getHeight(gapp->window)},
#else
    .imageExtent = caps.currentExtent,
#endif
    .imageArrayLayers = 1,
    .imageUsage = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT | VK_IMAGE_USAGE_TRANSFER_DST_BIT,
    #ifdef IDENTITY_PRETRANSFORM
    .preTransform = VK_SURFACE_TRANSFORM_IDENTITY_BIT_KHR,
#else
    .preTransform = caps.currentTransform,
#endif
    .compositeAlpha = VK_COMPOSITE_ALPHA_INHERIT_BIT_KHR,
    .presentMode = VK_PRESENT_MODE_FIFO_KHR, .clipped = 1, .oldSwapchain = old};
  if (!(caps.supportedCompositeAlpha & ci.compositeAlpha)) ci.compositeAlpha = VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR;
  LOG("recreate #%d (%s) req=%ux%u extent=%ux%u transform=%d -> vkCreateSwapchainKHR", ++recreates, why, ci.imageExtent.width, ci.imageExtent.height, caps.currentExtent.width, caps.currentExtent.height, caps.currentTransform);
  CK(vkCreateSwapchainKHR(dev, &ci, 0, &sc));
  LOG("recreate #%d done", recreates);
#ifdef WGPU_SYNC
  LOG("wgpusync drops=%d", syncDrops);
#endif
  if (old) vkDestroySwapchainKHR(dev, old, 0);
  nimg = 8; vkGetSwapchainImagesKHR(dev, sc, &nimg, imgs);
}

static void frame(struct android_app *app) {
  vkWaitForFences(dev, 1, &fence, 1, UINT64_MAX);
#ifdef WGPU_SYNC
  uint32_t i; VkResult r = vkAcquireNextImageKHR(dev, sc, UINT64_MAX, acq, afence, &i);
  if (r == VK_SUCCESS || r == VK_SUBOPTIMAL_KHR) { vkWaitForFences(dev, 1, &afence, 1, UINT64_MAX); vkResetFences(dev, 1, &afence); }
#else
  uint32_t i; VkResult r = vkAcquireNextImageKHR(dev, sc, UINT64_MAX, acq, 0, &i);
#endif
  if (r == VK_ERROR_OUT_OF_DATE_KHR) { makeSwapchain("acquire out-of-date"); return; }
#ifdef IDENTITY_PRETRANSFORM
  if (r == VK_SUBOPTIMAL_KHR) { static int n; if (++n % 30 == 1) LOG("acquire suboptimal x%d", n); }
#endif
  vkResetFences(dev, 1, &fence);
  VkCommandBufferAllocateInfo ba = {VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO, .commandPool = pool, .level = VK_COMMAND_BUFFER_LEVEL_PRIMARY, .commandBufferCount = 1};
  static VkCommandBuffer cb; if (!cb) vkAllocateCommandBuffers(dev, &ba, &cb);
  vkResetCommandBuffer(cb, 0);
  VkCommandBufferBeginInfo bi = {VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO};
  vkBeginCommandBuffer(cb, &bi);
  VkImageSubresourceRange rg = {VK_IMAGE_ASPECT_COLOR_BIT, 0, 1, 0, 1};
  VkImageMemoryBarrier b = {VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER, .dstAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT,
    .oldLayout = VK_IMAGE_LAYOUT_UNDEFINED, .newLayout = VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
    .srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED, .dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED, .image = imgs[i], .subresourceRange = rg};
  vkCmdPipelineBarrier(cb, VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, 0, 0, 0, 0, 0, 1, &b);
  float t = (frames % 120) / 120.0f;
  VkClearColorValue c = {{t, 0.3f, 1 - t, 1}};
  vkCmdClearColorImage(cb, imgs[i], VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, &c, 1, &rg);
  b.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT; b.dstAccessMask = 0;
  b.oldLayout = VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL; b.newLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR;
  vkCmdPipelineBarrier(cb, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT, 0, 0, 0, 0, 0, 1, &b);
  vkEndCommandBuffer(cb);
  VkPipelineStageFlags ws = VK_PIPELINE_STAGE_TRANSFER_BIT;
  VkSubmitInfo si = {VK_STRUCTURE_TYPE_SUBMIT_INFO, .waitSemaphoreCount = 1, .pWaitSemaphores = &acq, .pWaitDstStageMask = &ws,
    .commandBufferCount = 1, .pCommandBuffers = &cb, .signalSemaphoreCount = 1, .pSignalSemaphores = &done};
  vkQueueSubmit(q, 1, &si, fence);
  VkPresentInfoKHR pi = {VK_STRUCTURE_TYPE_PRESENT_INFO_KHR, .waitSemaphoreCount = 1, .pWaitSemaphores = &done, .swapchainCount = 1, .pSwapchains = &sc, .pImageIndices = &i};
  r = vkQueuePresentKHR(q, &pi);
#ifdef WAIT_REDRAW
  pthread_mutex_lock(&mu); presents++; pthread_cond_broadcast(&cv); pthread_mutex_unlock(&mu);
#endif
  if (++frames % 300 == 0) LOG("alive frames=%d", frames);
  if (r == VK_ERROR_OUT_OF_DATE_KHR) makeSwapchain("present out-of-date");
  else if (r == VK_SUBOPTIMAL_KHR) makeSwapchain("present suboptimal");
}

static void onCmd(struct android_app *app, int32_t cmd) {
  switch (cmd) {
  case APP_CMD_INIT_WINDOW: {
    LOG("INIT_WINDOW %p", app->window);
    if (!inst) initDevice();
    VkAndroidSurfaceCreateInfoKHR ci = {VK_STRUCTURE_TYPE_ANDROID_SURFACE_CREATE_INFO_KHR, .window = app->window};
    CK(vkCreateAndroidSurfaceKHR(inst, &ci, 0, &surf));
    sc = VK_NULL_HANDLE; makeSwapchain("init window"); ready = 1; break; }
  case APP_CMD_TERM_WINDOW:
    LOG("TERM_WINDOW"); ready = 0; vkDeviceWaitIdle(dev);
    if (sc) vkDestroySwapchainKHR(dev, sc, 0); sc = VK_NULL_HANDLE;
    if (surf) vkDestroySurfaceKHR(inst, surf, 0); surf = VK_NULL_HANDLE; break;
  case APP_CMD_WINDOW_RESIZED: LOG("WINDOW_RESIZED"); if (ready) makeSwapchain("window resized"); break;
  case APP_CMD_CONFIG_CHANGED: LOG("CONFIG_CHANGED"); if (ready) makeSwapchain("config changed"); break;
  case APP_CMD_DESTROY: LOG("DESTROY"); break;
  }
}

void android_main(struct android_app *app) {
  gapp = app; app->onAppCmd = onCmd; LOG("android_main start");
#ifdef WAIT_REDRAW
  app->activity->callbacks->onNativeWindowRedrawNeeded = redrawNeeded;
#endif
  while (!app->destroyRequested) {
    int ev; struct android_poll_source *src;
    while (ALooper_pollOnce(ready ? 0 : -1, 0, &ev, (void **)&src) >= 0) {
      if (src) src->process(app, src);
      if (app->destroyRequested) break;
    }
    if (ready && !app->destroyRequested) frame(app);
  }
  LOG("android_main exit");
}
