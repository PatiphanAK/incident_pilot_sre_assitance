import { authService } from './services/auth.service';
import { stockService } from './services/stock.service';
import { orderService } from './services/order.service';

// ผูก Service เข้ากับตัวแปร window เพื่อให้ HTML เรียกใช้ได้ตรงๆ
(window as any).api = {
    auth: authService,
    stock: stockService,
    order: orderService
};

console.log("✅ Services loaded into window.api");
