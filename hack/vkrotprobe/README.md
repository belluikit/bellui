# vkrotprobe (#909, not for merge)

A plain C Vulkan NativeActivity (no bellui, no wgpu) used to show that the
pixel143 rotation ANR is an emulator bug.

- `./build.sh <name> '<manifest attrs>'` with `EXTRA="-DWGPU_SYNC -DWAIT_REDRAW"`
  builds the variant that hangs: it acquires with a fence and, like wgpu-hal,
  destroys and recreates that fence and its semaphores on every swapchain
  recreate. Use bellui's configChanges, `locale|keyboard|keyboardHidden|orientation|screenLayout|uiMode|layoutDirection`.
- `rot.sh <apk> <pkg> <label> [n]` rotates n times (`ACT=android.app.NativeActivity`
  for this app), aborts as CONTAMINATED if anything else takes focus, and on a
  hang samples the emulator's qemu process (`sample`), which shows the host
  stall: `on_vkDestroySemaphore → DeviceOpTracker::PollAndProcessGarbage →
  Poll → vkGetFenceStatus`.
- `emurestart.sh` restarts only the pixel143 qemu on port 5554.

`experiments/` holds the two ruled-out changes to bellui main (cb1d0ab2), as
patches: `e1` drops `CopySrc` from the surface usage (hung at rotation 24),
`e2` drains the device and waits 100 ms before every reconfigure (hung at
rotations 3, 19 and 9).
