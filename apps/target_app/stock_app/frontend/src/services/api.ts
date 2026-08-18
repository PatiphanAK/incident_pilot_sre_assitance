import axios, { type AxiosError, type AxiosInstance } from "axios";

export class ApiError extends Error {
  readonly status: number;
  readonly data: unknown;

  constructor(message: string, status: number, data?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.data = data;
  }
}

// ต้องมีบรรทัดนี้นะครับ ห้ามลบหายไป (ใช้ค่าว่างเพื่อให้วิ่งเข้า Proxy)
const API_URL = "http://13.212.197.248:8080/";

export const apiClient: AxiosInstance = axios.create({
  baseURL: API_URL,
  withCredentials: false,
});

let authToken: string | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
}

apiClient.interceptors.request.use((config) => {
  if (authToken) {
    config.headers.Authorization = `Bearer ${authToken}`;
  }
  return config;
});

function extractMessage(data: unknown, fallback: string): string {
  if (data && typeof data === "object") {
    const obj = data as Record<string, unknown>;
    if (typeof obj.error === "string" && obj.error) return obj.error;
  }
  if (typeof data === "string" && data) return data;
  return fallback;
}

apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    const status = error.response?.status ?? 0;
    const data = error.response?.data;
    const fallback =
      status === 0
        ? "ไม่สามารถเชื่อมต่อกับ Backend ได้"
        : error.code === "ERR_NETWORK"
          ? "เครือข่ายมีปัญหา กรุณาลองใหม่"
          : "เกิดข้อผิดพลาดจากเซิร์ฟเวอร์";
    throw new ApiError(extractMessage(data, fallback), status, data);
  },
);
