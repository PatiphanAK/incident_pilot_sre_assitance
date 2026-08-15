// URL ของ Go Backend (สมมติว่ารันที่พอร์ต 8080)
const AUTH_API_URL = 'http://127.0.0.1:8080/api/auth/login';
let jwtToken = '';

document.getElementById('loginBtn').addEventListener('click', async () => {
    const email = document.getElementById('emailInput').value;
    const password = document.getElementById('passwordInput').value;
    const navStatus = document.getElementById('navStatus');

    navStatus.innerText = "⏳ กำลังล็อกอิน...";

    try {
        const response = await fetch(AUTH_API_URL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });

        const data = await response.json();

        if (response.ok) {
            jwtToken = data.token;
            document.getElementById('tokenDisplay').innerText = `✅ Token: ${jwtToken.substring(0, 15)}...`;
            document.getElementById('tokenDisplay').classList.replace('alert-secondary', 'alert-success');
            navStatus.innerText = `🟢 Logged In (${data.user.username})`;

            loadProducts(); // ล็อกอินผ่านแล้วค่อยโชว์สินค้า
        } else {
            alert(`เข้าสู่ระบบไม่สำเร็จ: ${data.error}`);
            navStatus.innerText = "🔴 Not Logged In";
        }
    } catch (error) {
        alert("เชื่อมต่อ Backend ไม่ได้ กรุณาเช็คว่ารันเซิร์ฟเวอร์ Go หรือยัง");
        navStatus.innerText = "🔴 Not Logged In";
    }
});

// ฟังก์ชันโชว์สินค้าและสั่งซื้อจริง
function loadProducts() {
    const productList = document.getElementById('productList');
    productList.innerHTML = `
        <div class="card bg-light border-0 shadow-sm mx-auto" style="max-width: 300px;">
            <div class="card-body text-center">
                <h3 class="card-title">Pencil 2B</h3>
                <h2 class="text-primary my-3">฿15.50</h2>
                <button id="buyBtn" class="btn btn-success w-100 fw-bold">🛒 สั่งซื้อ 1 ชิ้น</button>
            </div>
        </div>
    `;

    document.getElementById('buyBtn').addEventListener('click', async () => {
        const orderStatus = document.getElementById('orderStatus');
        orderStatus.innerHTML = `<br>⏳ [Order] กำลังส่งคำสั่งซื้อไปยังระบบ...`;

        try {
            // ดึง Token ที่ได้จากการ Login มาใช้ยืนยันตัวตน
            const token = localStorage.getItem('token');

            const response = await fetch('http://127.0.0.1:8080/api/orders', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                },
                body: JSON.stringify({ productId: "pencil-2b", quantity: 1 })
            });

            if (response.ok) {
                orderStatus.innerHTML = `<br>✅ [Order] สั่งซื้อสินค้าสำเร็จ! ตัดสต็อกเรียบร้อย`;
            } else {
                const errData = await response.json();
                orderStatus.innerHTML = `<br>❌ [Order] สั่งซื้อไม่สำเร็จ: ${errData.error || 'Unknown error'}`;
            }
        } catch (error) {
            orderStatus.innerHTML = `<br>❌ [Order] ไม่สามารถเชื่อมต่อกับเซิร์ฟเวอร์หลังบ้านได้`;
        }
    });
}
