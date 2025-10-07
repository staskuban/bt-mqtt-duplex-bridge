import can
bus = can.interface.Bus(bustype='socketcan', channel='can0')
msg = can.Message(arbitration_id=0x7DF, data=[0x02, 0x01, 0x0C, 0, 0, 0, 0, 0], is_extended_id=False)
bus.send(msg)