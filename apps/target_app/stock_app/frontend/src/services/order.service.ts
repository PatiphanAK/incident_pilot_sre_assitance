import { apiClient } from "./api";

export interface OrderItemInput {
  product_id: string;
  quantity: number;
}

export interface PlaceOrderDto {
  user_id?: string;
  items: OrderItemInput[];
}

export interface Order {
  id: string;
  user_id: string;
  total_amount: number;
  status: string;
}

export const orderService = {
  async placeOrder(dto: PlaceOrderDto): Promise<Order> {
    const response = await apiClient.post<Order>("/api/v1/orders", dto);
    return response.data;
  },

  async listOrders(): Promise<Order[]> {
    const response = await apiClient.get<Order[]>("/api/v1/orders");
    return response.data;
  },

  async getOrder(id: string): Promise<Order> {
    const response = await apiClient.get<Order>(`/api/v1/orders/${id}`);
    return response.data;
  },

  updateOrderStatus: async (id: string, status: string) => {
      // status ที่ส่งได้คือ PENDING, PAID, SHIPPED, CANCELLED[cite: 1]
      return await fetchAPI(`/api/v1/orders/${id}`, {
          method: 'PATCH',
          body: JSON.stringify({ status })
      });
  },

  deleteOrder: async (id: string) => {
      return await fetchAPI(`/api/v1/orders/${id}`, {
          method: 'DELETE'
      });
  }
};
