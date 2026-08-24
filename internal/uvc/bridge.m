//go:build darwin && cgo

#import "bridge.h"

#import <CoreFoundation/CoreFoundation.h>
#import <IOKit/IOCFPlugIn.h>
#import <IOKit/IOKitLib.h>
#import <IOKit/usb/IOUSBLib.h>
#import <IOKit/usb/IOUSBHostFamilyDefinitions.h>

#include <mach/mach_error.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>

enum {
    UVCVideoClass = 0x0e,
    UVCVideoControlSubclass = 0x01,
    UVCClassInterfaceDescriptor = 0x24,
    UVCVideoControlHeader = 0x01,
    UVCInputTerminal = 0x02,
    UVCSetCurrent = 0x01,
};

struct UVCBridgeController {
    IOUSBDeviceInterface **device;
    IOUSBInterfaceInterface220 **interface;
    uint8_t interface_number;
    uint8_t terminal_id;
    int interface_open;
};

static void UVCSetError(char **error_out, const char *format, ...) {
    if (error_out == NULL) {
        return;
    }

    va_list args;
    va_start(args, format);
    if (vasprintf(error_out, format, args) < 0) {
        *error_out = NULL;
    }
    va_end(args);
}

static uint32_t UVCNumberProperty(io_service_t service, CFStringRef key) {
    CFTypeRef property = IORegistryEntryCreateCFProperty(service, key, kCFAllocatorDefault, 0);
    if (property == NULL) {
        return 0;
    }

    int64_t value = 0;
    if (CFGetTypeID(property) == CFNumberGetTypeID()) {
        CFNumberGetValue((CFNumberRef)property, kCFNumberSInt64Type, &value);
    }
    CFRelease(property);
    return (uint32_t)value;
}

static IOUSBDeviceInterface **UVCCreateDeviceInterface(
    uint32_t location_id,
    uint16_t vendor_id,
    uint16_t product_id,
    IOReturn *failure_out
) {
    const char *class_names[] = {kIOUSBHostDeviceClassName, kIOUSBDeviceClassName};
    IOReturn last_failure = kIOReturnNotFound;

    for (size_t class_index = 0; class_index < sizeof(class_names) / sizeof(class_names[0]); class_index++) {
        CFMutableDictionaryRef matching = IOServiceMatching(class_names[class_index]);
        if (matching == NULL) {
            continue;
        }

        io_iterator_t iterator = IO_OBJECT_NULL;
        IOReturn result = IOServiceGetMatchingServices(kIOMainPortDefault, matching, &iterator);
        if (result != kIOReturnSuccess) {
            last_failure = result;
            continue;
        }

        io_service_t service;
        while ((service = IOIteratorNext(iterator)) != IO_OBJECT_NULL) {
            uint32_t found_vendor = UVCNumberProperty(service, CFSTR(kUSBVendorID));
            uint32_t found_product = UVCNumberProperty(service, CFSTR(kUSBProductID));
            uint32_t found_location = UVCNumberProperty(service, CFSTR(kUSBDevicePropertyLocationID));
            if (found_vendor != vendor_id || found_product != product_id || found_location != location_id) {
                IOObjectRelease(service);
                continue;
            }

            IOCFPlugInInterface **plugin = NULL;
            SInt32 score = 0;
            result = IOCreatePlugInInterfaceForService(
                service,
                kIOUSBDeviceUserClientTypeID,
                kIOCFPlugInInterfaceID,
                &plugin,
                &score
            );
            IOObjectRelease(service);
            if (result != kIOReturnSuccess || plugin == NULL) {
                last_failure = result;
                continue;
            }

            IOUSBDeviceInterface **device = NULL;
            HRESULT query_result = (*plugin)->QueryInterface(
                plugin,
                CFUUIDGetUUIDBytes(kIOUSBDeviceInterfaceID),
                (LPVOID *)&device
            );
            (*plugin)->Release(plugin);
            if (query_result == S_OK && device != NULL) {
                IOObjectRelease(iterator);
                return device;
            }
            last_failure = (IOReturn)query_result;
        }
        IOObjectRelease(iterator);
    }

    if (failure_out != NULL) {
        *failure_out = last_failure;
    }
    return NULL;
}

static void UVCReadTerminal(
    IOUSBInterfaceInterface220 **interface,
    uint8_t *terminal_id_out,
    uint32_t *controls_out
) {
    *terminal_id_out = 1;
    *controls_out = 0;

    IOUSBDescriptorHeader *current = NULL;
    const uint8_t *header = NULL;
    while ((current = (*interface)->FindNextAssociatedDescriptor(
                interface,
                current,
                UVCClassInterfaceDescriptor
            )) != NULL) {
        const uint8_t *bytes = (const uint8_t *)current;
        if (bytes[0] >= 7 && bytes[2] == UVCVideoControlHeader) {
            header = bytes;
            break;
        }
    }
    if (header == NULL) {
        return;
    }

    size_t total_length = (size_t)header[5] | ((size_t)header[6] << 8);
    for (size_t offset = 0; offset < total_length;) {
        const uint8_t *descriptor = header + offset;
        size_t length = descriptor[0];
        if (length == 0 || offset + length > total_length) {
            return;
        }
        if (length >= 15 && descriptor[1] == UVCClassInterfaceDescriptor && descriptor[2] == UVCInputTerminal) {
            size_t control_size = descriptor[14];
            if (15 + control_size > length) {
                return;
            }

            uint32_t controls = 0;
            for (size_t i = 0; i < control_size && i < sizeof(controls); i++) {
                controls |= (uint32_t)descriptor[15 + i] << (8 * i);
            }
            if (descriptor[3] != 0) {
                *terminal_id_out = descriptor[3];
            }
            *controls_out = controls;
            return;
        }
        offset += length;
    }
}

int uvc_open(
    uint32_t location_id,
    uint16_t vendor_id,
    uint16_t product_id,
    UVCBridgeController **controller_out,
    uint32_t *controls_out,
    char **error_out
) {
    if (controller_out == NULL || controls_out == NULL) {
        UVCSetError(error_out, "invalid controller output pointers");
        return 0;
    }
    *controller_out = NULL;
    *controls_out = 0;

    IOReturn result = kIOReturnSuccess;
    IOUSBDeviceInterface **device = UVCCreateDeviceInterface(location_id, vendor_id, product_id, &result);
    if (device == NULL) {
        UVCSetError(
            error_out,
            "USB device %04x:%04x at location 0x%08x not found (%s, 0x%08x)",
            vendor_id,
            product_id,
            location_id,
            mach_error_string(result),
            result
        );
        return 0;
    }

    IOUSBFindInterfaceRequest request = {
        UVCVideoClass,
        UVCVideoControlSubclass,
        kIOUSBFindInterfaceDontCare,
        kIOUSBFindInterfaceDontCare,
    };
    io_iterator_t iterator = IO_OBJECT_NULL;
    result = (*device)->CreateInterfaceIterator(device, &request, &iterator);
    if (result != kIOReturnSuccess) {
        (*device)->Release(device);
        UVCSetError(error_out, "find VideoControl interface: %s (0x%08x)", mach_error_string(result), result);
        return 0;
    }

    io_service_t service = IOIteratorNext(iterator);
    IOObjectRelease(iterator);
    if (service == IO_OBJECT_NULL) {
        (*device)->Release(device);
        UVCSetError(error_out, "USB device has no UVC VideoControl interface");
        return 0;
    }

    IOCFPlugInInterface **plugin = NULL;
    SInt32 score = 0;
    result = IOCreatePlugInInterfaceForService(
        service,
        kIOUSBInterfaceUserClientTypeID,
        kIOCFPlugInInterfaceID,
        &plugin,
        &score
    );
    IOObjectRelease(service);
    if (result != kIOReturnSuccess || plugin == NULL) {
        (*device)->Release(device);
        UVCSetError(error_out, "create VideoControl interface: %s (0x%08x)", mach_error_string(result), result);
        return 0;
    }

    IOUSBInterfaceInterface220 **interface = NULL;
    HRESULT query_result = (*plugin)->QueryInterface(
        plugin,
        CFUUIDGetUUIDBytes(kIOUSBInterfaceInterfaceID220),
        (LPVOID *)&interface
    );
    (*plugin)->Release(plugin);
    if (query_result != S_OK || interface == NULL) {
        (*device)->Release(device);
        UVCSetError(error_out, "query VideoControl interface: 0x%08x", (unsigned int)query_result);
        return 0;
    }

    uint8_t interface_number = 0;
    result = (*interface)->GetInterfaceNumber(interface, &interface_number);
    if (result != kIOReturnSuccess) {
        (*interface)->Release(interface);
        (*device)->Release(device);
        UVCSetError(error_out, "read VideoControl interface number: %s (0x%08x)", mach_error_string(result), result);
        return 0;
    }

    int interface_open = 0;
    result = (*interface)->USBInterfaceOpen(interface);
    if (result == kIOReturnSuccess) {
        interface_open = 1;
    } else if (result != kIOReturnExclusiveAccess) {
        (*interface)->Release(interface);
        (*device)->Release(device);
        UVCSetError(error_out, "open VideoControl interface: %s (0x%08x)", mach_error_string(result), result);
        return 0;
    }

    UVCBridgeController *controller = calloc(1, sizeof(UVCBridgeController));
    if (controller == NULL) {
        if (interface_open) {
            (*interface)->USBInterfaceClose(interface);
        }
        (*interface)->Release(interface);
        (*device)->Release(device);
        UVCSetError(error_out, "allocate UVC controller");
        return 0;
    }

    controller->device = device;
    controller->interface = interface;
    controller->interface_number = interface_number;
    controller->interface_open = interface_open;
    UVCReadTerminal(interface, &controller->terminal_id, controls_out);
    *controller_out = controller;
    return 1;
}

int uvc_control(
    UVCBridgeController *controller,
    uint8_t selector,
    uint8_t request,
    void *data,
    uint16_t length,
    char **error_out
) {
    if (controller == NULL || controller->interface == NULL || data == NULL || length == 0) {
        UVCSetError(error_out, "invalid control request");
        return 0;
    }

    IOUSBDevRequest control_request = {
        .bmRequestType = request == UVCSetCurrent ? 0x21 : 0xa1,
        .bRequest = request,
        .wValue = (uint16_t)selector << 8,
        .wIndex = ((uint16_t)controller->terminal_id << 8) | controller->interface_number,
        .wLength = length,
        .pData = data,
        .wLenDone = 0,
    };
    IOReturn result = (*controller->interface)->ControlRequest(controller->interface, 0, &control_request);
    if (result != kIOReturnSuccess) {
        UVCSetError(
            error_out,
            "control selector 0x%02x request 0x%02x: %s (0x%08x)",
            selector,
            request,
            mach_error_string(result),
            result
        );
        return 0;
    }
    if (control_request.wLenDone != length) {
        UVCSetError(
            error_out,
            "control selector 0x%02x request 0x%02x transferred %u of %u bytes",
            selector,
            request,
            control_request.wLenDone,
            length
        );
        return 0;
    }
    return 1;
}

void uvc_close(UVCBridgeController *controller) {
    if (controller == NULL) {
        return;
    }
    if (controller->interface != NULL) {
        if (controller->interface_open) {
            (*controller->interface)->USBInterfaceClose(controller->interface);
        }
        (*controller->interface)->Release(controller->interface);
    }
    if (controller->device != NULL) {
        (*controller->device)->Release(controller->device);
    }
    free(controller);
}
