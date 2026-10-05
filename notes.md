AMD Posted an email about a driver for the lightbar on their AMD Ryzen Halo developer box.

Subject: [PATCH] platform/x86/amd: Introduce Halo Box RGB LED driver
Date: Sun, 26 Apr 2026 21:25:46 -0500	[thread overview]
Message-ID: <20260427022546.1407923-1-superm1@kernel.org> (raw)

The Halo Box features an RGB LED light bar that can be controlled
through WMI methods to display any color combination.

The driver exposes the LED through the LED multicolor subsystem,
allowing userspace to control RGB values via sysfs:
  /sys/class/leds/amd_halo:rgb:light_bar/multi_intensity
  /sys/class/leds/amd_halo:rgb:light_bar/brightness

Hardware interface:
- Three separate RGB channels (Red, Green, Blue)
- Each channel controlled via individual WMI method calls
- Value range: 0-100 (matching hardware range directly)

Co-developed-by: Leo.Lin@amd.com
Signed-off-by: Leo.Lin@amd.com
Signed-off-by: Mario Limonciello (AMD) <superm1@kernel.org>
---
 MAINTAINERS                             |   7 +
 drivers/platform/x86/amd/Kconfig        |  12 +
 drivers/platform/x86/amd/Makefile       |   1 +
 drivers/platform/x86/amd/amd_halo_led.c | 375 ++++++++++++++++++++++++
 4 files changed, 395 insertions(+)
 create mode 100644 drivers/platform/x86/amd/amd_halo_led.c

diff --git a/MAINTAINERS b/MAINTAINERS
index 2fb1c75afd163..c35b654a65041 100644
--- a/MAINTAINERS
+++ b/MAINTAINERS
@@ -1124,6 +1124,13 @@ F:	drivers/char/hw_random/geode-rng.c
 F:	drivers/crypto/geode*
 F:	drivers/video/fbdev/geode/
 
+AMD HALO BOX RGB LED DRIVER
+M:	Mario Limonciello (AMD) <superm1@kernel.org>
+R:	Leo Lin <Leo.Lin@amd.com>
+L:	platform-driver-x86@vger.kernel.org
+S:	Supported
+F:	drivers/platform/x86/amd/amd_halo_led.c
+
 AMD HSMP DRIVER
 M:	Naveen Krishna Chatradhi <naveenkrishna.chatradhi@amd.com>
 R:	Carlos Bilbao <carlos.bilbao@kernel.org>
diff --git a/drivers/platform/x86/amd/Kconfig b/drivers/platform/x86/amd/Kconfig
index b813f92653686..d642386938055 100644
--- a/drivers/platform/x86/amd/Kconfig
+++ b/drivers/platform/x86/amd/Kconfig
@@ -34,6 +34,18 @@ config AMD_WBRF
 	  This mechanism will only be activated on platforms that advertise a
 	  need for it.
 
+config AMD_HALO_LED
+	tristate "AMD Halo Box RGB LED Driver"
+	depends on ACPI_WMI
+	select LEDS_CLASS_MULTICOLOR
+	help
+	  This driver provides RGB LED control for AMD Halo Box devices
+	  through the LED multicolor subsystem. The Halo Box light bar can
+	  be controlled via sysfs to display any RGB color combination.
+
+	  To compile this driver as a module, choose M here: the module
+	  will be called amd_halo_led.
+
 config AMD_ISP_PLATFORM
 	tristate "AMD ISP4 platform driver"
 	depends on I2C && X86_64 && ACPI
diff --git a/drivers/platform/x86/amd/Makefile b/drivers/platform/x86/amd/Makefile
index f6ff0c837f345..2f467dbbfc8a3 100644
--- a/drivers/platform/x86/amd/Makefile
+++ b/drivers/platform/x86/amd/Makefile
@@ -10,5 +10,6 @@ obj-$(CONFIG_AMD_PMC)		+= pmc/
 obj-$(CONFIG_AMD_HSMP)		+= hsmp/
 obj-$(CONFIG_AMD_PMF)		+= pmf/
 obj-$(CONFIG_AMD_WBRF)		+= wbrf.o
+obj-$(CONFIG_AMD_HALO_LED)	+= amd_halo_led.o
 obj-$(CONFIG_AMD_ISP_PLATFORM)	+= amd_isp4.o
 obj-$(CONFIG_AMD_HFI)		+= hfi/
diff --git a/drivers/platform/x86/amd/amd_halo_led.c b/drivers/platform/x86/amd/amd_halo_led.c
new file mode 100644
index 0000000000000..01b4583ca70f3
--- /dev/null
+++ b/drivers/platform/x86/amd/amd_halo_led.c
@@ -0,0 +1,375 @@
+// SPDX-License-Identifier: GPL-2.0
+/*
+ * AMD Halo Box RGB LED Driver
+ *
+ * Copyright (C) 2026 Advanced Micro Devices, Inc.
+ *
+ * This driver provides RGB LED control for AMD Halo Box devices through
+ * the LED multicolor subsystem. The Halo Box light bar can be controlled
+ * via sysfs to display any RGB color combination.
+ */
+
+#include <linux/acpi.h>
+#include <linux/led-class-multicolor.h>
+#include <linux/leds.h>
+#include <linux/module.h>
+#include <linux/mutex.h>
+#include <linux/wmi.h>
+
+#define AMD_HALO_GUID "081E747B-E028-4232-AF24-EAAEAB2B1E86"
+
+/* WMI method IDs from MOF */
+#define AMD_HALO_WMI_GET_LIGHTBAR	0x01
+#define AMD_HALO_WMI_SET_LIGHTBAR	0x02
+#define AMD_HALO_WMI_TURN_ON		0x03
+#define AMD_HALO_WMI_TURN_OFF		0x04
+
+/* Channel selectors for Arg0 */
+#define AMD_HALO_CHANNEL_RED		0x01
+#define AMD_HALO_CHANNEL_GREEN		0x02
+#define AMD_HALO_CHANNEL_BLUE		0x03
+
+/* Status codes from spec */
+#define AMD_HALO_STATUS_SUCCESS		0x0000
+#define AMD_HALO_STATUS_INVALID_PARAM	0xFFFD
+
+/* Brightness uses 0-100 range */
+#define AMD_HALO_MAX_HW_BRIGHTNESS		100
+
+/**
+ * struct amd_halo_led_data - Driver private data
+ * @wdev: WMI device pointer
+ * @led_mc: LED multicolor class device
+ * @subled_info: RGB channel information
+ * @lock: Mutex to protect WMI calls
+ */
+struct amd_halo_led_data {
+	struct wmi_device *wdev;
+	struct led_classdev_mc led_mc;
+	struct mc_subled subled_info[3];
+	struct mutex lock;	/* Protects WMI method calls */
+};
+
+struct amd_halo_wmi_args {
+	u32 arg0;
+	u32 arg1;
+};
+
+/**
+ * amd_halo_wmi_set_channel - Set a single RGB channel value
+ * @wdev: WMI device pointer
+ * @channel: Channel selector (1=Red, 2=Green, 3=Blue)
+ * @brightness: brightness to set (0-100)
+ *
+ * Return: 0 on success, negative error code on failure
+ */
+static int amd_halo_wmi_set_channel(struct wmi_device *wdev, u32 channel,
+				    u32 brightness)
+{
+	struct amd_halo_wmi_args args = {
+		.arg0 = channel,
+		.arg1 = brightness,
+	};
+	const struct acpi_buffer input = {
+		.length = sizeof(args),
+		.pointer = &args,
+	};
+	struct acpi_buffer output = { ACPI_ALLOCATE_BUFFER, NULL };
+	union acpi_object *obj;
+	acpi_status status;
+	u16 result_status;
+	int ret = 0;
+
+	/* Validate input per spec */
+	if (channel < AMD_HALO_CHANNEL_RED ||
+	    channel > AMD_HALO_CHANNEL_BLUE ||
+	    brightness > AMD_HALO_MAX_HW_BRIGHTNESS)
+		return -EINVAL;
+
+	status = wmidev_evaluate_method(wdev, 0, AMD_HALO_WMI_SET_LIGHTBAR,
+					&input, &output);
+	if (ACPI_FAILURE(status)) {
+		dev_err(&wdev->dev, "SetLightBar failed: %s\n",
+			acpi_format_exception(status));
+		return -EIO;
+	}
+
+	/* Parse return buffer per spec: Bytes[0:1] = Status */
+	obj = output.pointer;
+	if (!obj || obj->type != ACPI_TYPE_BUFFER || obj->buffer.length < 2 ||
+	    !obj->buffer.pointer) {
+		dev_err(&wdev->dev, "Invalid return buffer\n");
+		ret = -EIO;
+		goto out;
+	}
+
+	result_status = obj->buffer.pointer[0] |
+			(obj->buffer.pointer[1] << 8);
+	if (result_status != AMD_HALO_STATUS_SUCCESS) {
+		dev_err(&wdev->dev, "WMI returned error: 0x%04x\n",
+			result_status);
+		ret = -EIO;
+	}
+
+out:
+	kfree(output.pointer);
+	return ret;
+}
+
+/**
+ * amd_halo_wmi_get_channel - Get a single RGB channel value
+ * @wdev: WMI device pointer
+ * @channel: Channel selector (1=Red, 2=Green, 3=Blue)
+ * @value: Pointer to store the read value
+ *
+ * Return: 0 on success, negative error code on failure
+ */
+static int amd_halo_wmi_get_channel(struct wmi_device *wdev, u32 channel,
+				    u8 *value)
+{
+	struct amd_halo_wmi_args args = {
+		.arg0 = channel,
+		.arg1 = 0,  /* Reserved */
+	};
+	const struct acpi_buffer input = {
+		.length = sizeof(args),
+		.pointer = &args,
+	};
+	struct acpi_buffer output = { ACPI_ALLOCATE_BUFFER, NULL };
+	union acpi_object *obj;
+	acpi_status status;
+	u16 result_status;
+	int ret = 0;
+
+	if (channel < AMD_HALO_CHANNEL_RED || channel > AMD_HALO_CHANNEL_BLUE)
+		return -EINVAL;
+
+	status = wmidev_evaluate_method(wdev, 0, AMD_HALO_WMI_GET_LIGHTBAR,
+					&input, &output);
+	if (ACPI_FAILURE(status))
+		return -EIO;
+
+	obj = output.pointer;
+	if (!obj || obj->type != ACPI_TYPE_BUFFER || obj->buffer.length < 3 ||
+	    !obj->buffer.pointer) {
+		ret = -EIO;
+		goto out;
+	}
+
+	result_status = obj->buffer.pointer[0] |
+			(obj->buffer.pointer[1] << 8);
+	if (result_status != AMD_HALO_STATUS_SUCCESS) {
+		ret = -EIO;
+		goto out;
+	}
+
+	/* Value returned in Byte[2] per spec */
+	*value = obj->buffer.pointer[2];
+
+out:
+	kfree(output.pointer);
+	return ret;
+}
+
+/**
+ * amd_halo_wmi_turn_off - Turn off all LED channels
+ * @wdev: WMI device pointer
+ *
+ * Return: 0 on success, negative error code on failure
+ */
+static int amd_halo_wmi_turn_off(struct wmi_device *wdev)
+{
+	struct amd_halo_wmi_args args = {
+		.arg0 = 0,
+		.arg1 = 0,
+	};
+	const struct acpi_buffer input = {
+		.length = sizeof(args),
+		.pointer = &args,
+	};
+	struct acpi_buffer output = { ACPI_ALLOCATE_BUFFER, NULL };
+	acpi_status status;
+
+	status = wmidev_evaluate_method(wdev, 0, AMD_HALO_WMI_TURN_OFF,
+					&input, &output);
+	kfree(output.pointer);
+
+	return ACPI_FAILURE(status) ? -EIO : 0;
+}
+
+/**
+ * amd_halo_brightness_set - Set LED brightness and color
+ * @cdev: LED class device
+ * @brightness: Brightness value (0 = off, >0 = on with color)
+ *
+ * Return: 0 on success, negative error code on failure
+ */
+static int amd_halo_brightness_set(struct led_classdev *cdev,
+				   enum led_brightness brightness)
+{
+	struct led_classdev_mc *mc_cdev = lcdev_to_mccdev(cdev);
+	struct amd_halo_led_data *data = container_of(mc_cdev,
+						       struct amd_halo_led_data,
+						       led_mc);
+	u32 red_hw, green_hw, blue_hw;
+	int ret;
+
+	guard(mutex)(&data->lock);
+
+	if (brightness == 0)
+		return amd_halo_wmi_turn_off(data->wdev);
+
+	led_mc_calc_color_components(mc_cdev, brightness);
+	red_hw = mc_cdev->subled_info[0].brightness;
+	green_hw = mc_cdev->subled_info[1].brightness;
+	blue_hw = mc_cdev->subled_info[2].brightness;
+
+	/* Set each channel individually - 3 WMI calls required */
+	ret = amd_halo_wmi_set_channel(data->wdev, AMD_HALO_CHANNEL_RED,
+				       red_hw);
+	if (ret)
+		return ret;
+
+	ret = amd_halo_wmi_set_channel(data->wdev, AMD_HALO_CHANNEL_GREEN,
+				       green_hw);
+	if (ret)
+		return ret;
+
+	return amd_halo_wmi_set_channel(data->wdev, AMD_HALO_CHANNEL_BLUE,
+					blue_hw);
+}
+
+/**
+ * amd_halo_brightness_get - Get current LED brightness state
+ * @cdev: LED class device
+ *
+ * Return: brightness last set by the user, or AMD_HALO_MAX_HW_BRIGHTNESS if
+ *         it was set by BIOS, or LED_OFF if all LEDs are off.
+ */
+static enum led_brightness amd_halo_brightness_get(struct led_classdev *cdev)
+{
+	struct led_classdev_mc *mc_cdev = lcdev_to_mccdev(cdev);
+	struct amd_halo_led_data *data = container_of(mc_cdev,
+						       struct amd_halo_led_data,
+						       led_mc);
+	u8 red_hw = 0, green_hw = 0, blue_hw = 0;
+	int ret;
+
+	guard(mutex)(&data->lock);
+
+	/* Read each channel */
+	ret = amd_halo_wmi_get_channel(data->wdev, AMD_HALO_CHANNEL_RED,
+				       &red_hw);
+	if (ret)
+		return LED_OFF;
+
+	ret = amd_halo_wmi_get_channel(data->wdev, AMD_HALO_CHANNEL_GREEN,
+				       &green_hw);
+	if (ret)
+		return LED_OFF;
+
+	ret = amd_halo_wmi_get_channel(data->wdev, AMD_HALO_CHANNEL_BLUE,
+				       &blue_hw);
+	if (ret)
+		return LED_OFF;
+
+	if (mc_cdev->subled_info[0].brightness == red_hw
+	    && mc_cdev->subled_info[1].brightness == green_hw
+	    && mc_cdev->subled_info[2].brightness == blue_hw) {
+		return cdev->brightness;
+	}
+
+	mc_cdev->subled_info[0].intensity = red_hw;
+	mc_cdev->subled_info[1].intensity = green_hw;
+	mc_cdev->subled_info[2].intensity = blue_hw;
+
+	return AMD_HALO_MAX_HW_BRIGHTNESS;
+}
+
+/**
+ * amd_halo_color_set_default - Set LED to initial color and brightness
+ * @cdev: LED class device
+ *
+ * Sets all RGB channels to 20% intensity as an initial state.
+ *
+ * Return: 0 on success, negative error code on failure
+ */
+static int amd_halo_color_set_default(struct led_classdev *cdev)
+{
+	struct led_classdev_mc *mc_cdev = lcdev_to_mccdev(cdev);
+
+	mc_cdev->subled_info[0].intensity = AMD_HALO_MAX_HW_BRIGHTNESS / 5;
+	mc_cdev->subled_info[1].intensity = AMD_HALO_MAX_HW_BRIGHTNESS / 5;
+	mc_cdev->subled_info[2].intensity = AMD_HALO_MAX_HW_BRIGHTNESS / 5;
+
+	return amd_halo_brightness_set(cdev, AMD_HALO_MAX_HW_BRIGHTNESS);
+}
+
+/**
+ * amd_halo_probe - Driver probe function
+ * @wdev: WMI device
+ * @context: Context data (unused)
+ *
+ * Return: 0 on success, negative error code on failure
+ */
+static int amd_halo_probe(struct wmi_device *wdev, const void *context)
+{
+	struct amd_halo_led_data *data;
+	int ret;
+
+	data = devm_kzalloc(&wdev->dev, sizeof(*data), GFP_KERNEL);
+	if (!data)
+		return -ENOMEM;
+
+	data->wdev = wdev;
+	mutex_init(&data->lock);
+	dev_set_drvdata(&wdev->dev, data);
+
+	data->subled_info[0].color_index = LED_COLOR_ID_RED;
+	data->subled_info[1].color_index = LED_COLOR_ID_GREEN;
+	data->subled_info[2].color_index = LED_COLOR_ID_BLUE;
+	data->subled_info[0].intensity = 0;
+	data->subled_info[1].intensity = 0;
+	data->subled_info[2].intensity = 0;
+
+	data->led_mc.led_cdev.name = "amd_halo:rgb:light_bar";
+	data->led_mc.led_cdev.brightness = AMD_HALO_MAX_HW_BRIGHTNESS;
+	data->led_mc.led_cdev.max_brightness = AMD_HALO_MAX_HW_BRIGHTNESS;
+	data->led_mc.led_cdev.brightness_set_blocking = amd_halo_brightness_set;
+	data->led_mc.led_cdev.brightness_get = amd_halo_brightness_get;
+	data->led_mc.led_cdev.flags = LED_CORE_SUSPENDRESUME | LED_RETAIN_AT_SHUTDOWN;
+	data->led_mc.num_colors = 3;
+	data->led_mc.subled_info = data->subled_info;
+
+	ret = devm_led_classdev_multicolor_register(&wdev->dev, &data->led_mc);
+	if (ret)
+		return dev_err_probe(&wdev->dev, ret,
+				     "Failed to register multicolor LED\n");
+
+	ret = amd_halo_color_set_default(&data->led_mc.led_cdev);
+	if (ret)
+		dev_warn(&wdev->dev, "Unable to set default LED intensity through WMI: %d\n", ret);
+
+	return 0;
+}
+
+static const struct wmi_device_id amd_halo_id_table[] = {
+	{ .guid_string = AMD_HALO_GUID },
+	{ }
+};
+MODULE_DEVICE_TABLE(wmi, amd_halo_id_table);
+
+static struct wmi_driver amd_halo_driver = {
+	.driver = {
+		.name = "amd_halo_led",
+	},
+	.id_table = amd_halo_id_table,
+	.probe = amd_halo_probe,
+};
+
+module_wmi_driver(amd_halo_driver);
+
+MODULE_AUTHOR("Mario Limonciello (AMD) <superm1@kernel.org>");
+MODULE_AUTHOR("Leo Lin <Leo.Lin@amd.com>");
+MODULE_DESCRIPTION("AMD Halo Box RGB LED Control Driver");
+MODULE_LICENSE("GPL");
-- 