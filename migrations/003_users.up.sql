CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    avatar_url TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- สร้าง admin user เริ่มต้น (password: admin123)
INSERT INTO users (email, password_hash, name, role) VALUES 
('admin@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqzKzWJx5G5p5Q5Q5Q5Q5Q5Q5Q5Q5', 'Admin', 'admin')
ON CONFLICT (email) DO NOTHING;