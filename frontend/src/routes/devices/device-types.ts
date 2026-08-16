export interface Device {
  id?: number; // saved-device id, present for saved devices (enables delete)
  serial: string;
  model: string;
  brand: string;
  state: string;
  android_version: string;
}

export interface SavedDevice {
  id: number;
  address: string;
  name: string;
  last_connected_at: string;
  created_at: string;
}

export interface DeviceInfo {
  serial: string;
  model: string;
  brand: string;
  manufacturer: string;
  android_version: string;
  sdk_version: string;
  build_id: string;
  security_patch: string;
  magisk_version: string;
  ksu_version: string;
  apatch_version: string;
  battery_level: number;
  battery_status: string;
  storage_total: string;
  storage_used: string;
  storage_free: string;
  ram_total: string;
  ram_free: string;
  ram_used: string;
  uptime: string;
  kernel: string;
  abi: string;
}

export interface InstalledModule {
  name: string;
  version: string;
  author: string;
  description: string;
  enabled: boolean;
  size: string;
  source: string;
  update_date: string;
  has_update: boolean;
}

export interface AppInfo {
  app_name?: string;
  package_name: string;
  version_name: string;
  version_code: number;
  target_sdk: number;
  enabled: boolean;
  system: boolean;
}

export interface FileInfo {
  name: string;
  path: string;
  size: number;
  mode: string;
  is_dir: boolean;
}