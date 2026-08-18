import { apiClient, setAuthToken } from "./api";

export interface User {
  id: string;
  username: string;
  email: string;
}

export interface RegisterDto {
  username: string;
  email: string;
  password: string;
}

export interface LoginDto {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export const authService = {
  async register(dto: RegisterDto): Promise<User> {
    const response = await apiClient.post<User>("/api/v1/auth/register", dto);
    return response.data;
  },

  async login(dto: LoginDto): Promise<LoginResponse> {
    const response = await apiClient.post<LoginResponse>("/api/v1/auth/login", dto);
    if (response.data?.token) {
      setAuthToken(response.data.token);
    }
    return response.data;
  },

  async getMe(): Promise<User> {
    const response = await apiClient.get<User>("/api/v1/auth/me");
    return response.data;
  },



  logout() {
    setAuthToken(null);
  }
};
