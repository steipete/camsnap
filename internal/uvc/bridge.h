#ifndef CAMSNAP_UVC_BRIDGE_H
#define CAMSNAP_UVC_BRIDGE_H

#include <stdint.h>

typedef struct UVCBridgeController UVCBridgeController;

int uvc_open(
    uint32_t location_id,
    uint16_t vendor_id,
    uint16_t product_id,
    UVCBridgeController **controller_out,
    uint32_t *controls_out,
    char **error_out
);
int uvc_control(
    UVCBridgeController *controller,
    uint8_t selector,
    uint8_t request,
    void *data,
    uint16_t length,
    char **error_out
);
void uvc_close(UVCBridgeController *controller);

#endif
