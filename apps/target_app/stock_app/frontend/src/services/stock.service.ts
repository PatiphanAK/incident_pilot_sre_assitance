import { apiClient } from "./api";

export interface Product {
  id: string;
  name: string;
  quantity: number;
  price: number;
}

export interface CreateProductDto {
  name: string;
  quantity: number;
  price: number;
}

export const stockService = {
  async listProducts(): Promise<Product[]> {
    const response = await apiClient.get<Product[]>("/api/v1/stock/products");
    return response.data;
  },

  async getProduct(id: string): Promise<Product> {
    const response = await apiClient.get<Product>(`/api/v1/stock/products/${id}`);
    return response.data;
  },

  async createProduct(dto: CreateProductDto): Promise<Product> {
    const response = await apiClient.post<Product>("/api/v1/stock/products", dto);
    return response.data;
  },

  async reserve(id: string, quantity: number): Promise<{product_id: string, unit_price: number}> {
    const response = await apiClient.post(`/api/v1/stock/products/${id}/reserve`, { quantity });
    return response.data;
  },

  // ใน object ของ stock
  updateProduct: async (id: string, data: any) => {
    // ต้องมี price เสมอตามสเปค Backend
    return await fetchAPI(`/api/v1/stock/products/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data)
    });
  },

  deleteProduct: async (id: string) => {
      return await fetchAPI(`/api/v1/stock/products/${id}`, {
          method: 'DELETE'
      });
  },

  setInventory: async (id: string, quantity: number) => {
      return await fetchAPI(`/api/v1/stock/products/${id}/inventory`, {
          method: 'PATCH',
          body: JSON.stringify({ quantity }) // ส่ง quantity เป็นตัวเลขตรงๆ ไปเลย
      });
  }
};

export async function createProduct(name: string, price: number, quantity: number) {
    const token = localStorage.getItem('token');

    const response = await fetch('/api/v1/stock/products', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
            name: name,
            price: price,
            quantity: quantity
        })
    });

    if (!response.ok) {
        throw new Error('เกิดข้อผิดพลาดในการสร้างสินค้า');
    }

    return await response.json();
}
