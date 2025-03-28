package main

import "fmt"

func main() {
	for i := range 1500 {
		sql := fmt.Sprintf("INSERT INTO qrem_device.bed ( name, mac, third_device_pressure_pad_id, type, user_ids, this_city, this_address, latitude, longitude, create_time, create_by, update_time, update_by, del_flag, left_bind_user, right_bind_user, region, bed_end_light, bed_network, mattress_model, kernel_version, linux_app_version, algorithm_version, mcu_left_version, mcu_right_version, online_status) VALUES ( 'Jay-Bed', '25MM111111110038100000-%d', null, 0, '', '', null, null, null, '2025-03-24 11:39:49', 'admin', '2025-03-24 11:39:49', 'admin', 0, null, null, null, null, null, 'EK-E', '', '', '', '', '', 0);", i)
		fmt.Println(sql)

	}
}
