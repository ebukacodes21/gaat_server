CREATE TYPE user_role AS ENUM (
    'user',
    'staff',
    'supervisor',
    'admin'
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    role user_role NOT NULL DEFAULT 'user',
    verification_code VARCHAR(20),
    verification_code_expires_at TIMESTAMPTZ,
    account_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    last_login TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    address TEXT NOT NULL,        
    lga VARCHAR(100) NOT NULL,
    zip_code VARCHAR(20) NOT NULL,
    about_us VARCHAR(20) NOT NULL,
    state VARCHAR(100) NOT NULL,
    gender VARCHAR(50) NOT NULL,
    marital_status VARCHAR(50) NOT NULL,
    phone1 VARCHAR(30) NOT NULL,
    phone2 VARCHAR(30) NOT NULL,
    occupation VARCHAR(150) NOT NULL,
    terms_accepted BOOLEAN NOT NULL DEFAULT FALSE,
    img_url VARCHAR(512) NOT NULL DEFAULT 'https://randomuser.me/api/portraits/lego/1.jpg',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for lightning-fast lookups during login and authentication pipelines
CREATE INDEX idx_users_email ON users(email);

CREATE TABLE staffs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    role user_role DEFAULT 'staff',
    account_enabled BOOLEAN DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast lookup during login
CREATE INDEX idx_staffs_email ON staffs(email);