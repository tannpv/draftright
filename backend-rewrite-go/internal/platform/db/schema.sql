--
-- PostgreSQL database dump
--

\restrict Iw8RjWS8hHTbdhKdf8qIdIA6VincHu4mwbki5Enci8WJwJNminJ57HRkYSWPRaN

-- Dumped from database version 16.13
-- Dumped by pg_dump version 16.13

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: ai_providers_type_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.ai_providers_type_enum AS ENUM (
    'openai',
    'anthropic',
    'ollama',
    'custom'
);


--
-- Name: plans_billing_period_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.plans_billing_period_enum AS ENUM (
    'none',
    'monthly',
    'yearly'
);


--
-- Name: subscriptions_status_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.subscriptions_status_enum AS ENUM (
    'active',
    'cancelled',
    'expired'
);


--
-- Name: subscriptions_store_type_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.subscriptions_store_type_enum AS ENUM (
    'google_play',
    'apple_iap',
    'admin_granted',
    'lemonsqueezy',
    'stripe'
);


--
-- Name: users_auth_provider_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.users_auth_provider_enum AS ENUM (
    'local',
    'google',
    'facebook',
    'tiktok',
    'apple'
);


--
-- Name: users_role_enum; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.users_role_enum AS ENUM (
    'user'
);


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: admin_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.admin_users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255) NOT NULL,
    name character varying(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    role character varying(20) DEFAULT 'admin'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);

--
-- Name: admin_user_audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.admin_user_audit_log (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    actor_admin_id uuid NOT NULL,
    actor_email text NOT NULL,
    target_admin_id uuid NOT NULL,
    target_email text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ai_providers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_providers (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(255) NOT NULL,
    type public.ai_providers_type_enum NOT NULL,
    endpoint_url character varying(500) NOT NULL,
    api_key character varying(500) DEFAULT ''::character varying NOT NULL,
    model character varying(100) NOT NULL,
    temperature numeric(3,2) DEFAULT 0.3 NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: app_release_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.app_release_policies (
    platform character varying(20) NOT NULL,
    preferred character varying(20) DEFAULT 'direct'::character varying NOT NULL,
    store_status character varying(30) DEFAULT 'not_submitted'::character varying NOT NULL,
    notes text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: app_releases; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.app_releases (
    platform character varying(20) NOT NULL,
    version character varying(50) NOT NULL,
    download_url text NOT NULL,
    sha256 character varying(64) DEFAULT ''::character varying NOT NULL,
    release_notes text DEFAULT ''::text NOT NULL,
    required boolean DEFAULT false NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    channel character varying(20) DEFAULT 'direct'::character varying NOT NULL,
    enabled boolean DEFAULT true NOT NULL
);


--
-- Name: app_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.app_settings (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    environment character varying(20) DEFAULT 'testing'::character varying NOT NULL,
    trial_limit integer DEFAULT 3 NOT NULL,
    token_expiry_minutes integer DEFAULT 15 NOT NULL,
    max_input_length integer DEFAULT 3000 NOT NULL,
    supported_languages text DEFAULT 'Arabic,Chinese (Simplified),Chinese (Traditional),Czech,Danish,Dutch,English,Finnish,French,German,Greek,Hebrew,Hindi,Hungarian,Indonesian,Italian,Japanese,Korean,Malay,Norwegian,Polish,Portuguese,Romanian,Russian,Spanish,Swedish,Thai,Turkish,Ukrainian,Vietnamese'::text NOT NULL,
    stripe_secret_key character varying(500) DEFAULT ''::character varying NOT NULL,
    stripe_webhook_secret character varying(500) DEFAULT ''::character varying NOT NULL,
    paypal_client_id character varying(500) DEFAULT ''::character varying NOT NULL,
    paypal_client_secret character varying(500) DEFAULT ''::character varying NOT NULL,
    paypal_mode character varying(20) DEFAULT 'sandbox'::character varying NOT NULL,
    momo_partner_code character varying(500) DEFAULT ''::character varying NOT NULL,
    momo_access_key character varying(500) DEFAULT ''::character varying NOT NULL,
    momo_secret_key character varying(500) DEFAULT ''::character varying NOT NULL,
    momo_mode character varying(20) DEFAULT 'sandbox'::character varying NOT NULL,
    vietqr_bank_id character varying(20) DEFAULT 'MB'::character varying NOT NULL,
    vietqr_account_number character varying(100) DEFAULT ''::character varying NOT NULL,
    vietqr_account_name character varying(200) DEFAULT ''::character varying NOT NULL,
    casso_api_key character varying(500) DEFAULT ''::character varying NOT NULL,
    sepay_api_key character varying(500) DEFAULT ''::character varying NOT NULL,
    google_client_id character varying(500) DEFAULT '22951518033-gf853ftmf4emivffk0su2bik42j7cmai.apps.googleusercontent.com'::character varying NOT NULL,
    google_client_secret character varying(500) DEFAULT ''::character varying NOT NULL,
    apple_client_id character varying(500) DEFAULT ''::character varying NOT NULL,
    apple_team_id character varying(500) DEFAULT ''::character varying NOT NULL,
    apple_key_id character varying(500) DEFAULT ''::character varying NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    refresh_token_expiry_days integer DEFAULT 90 NOT NULL,
    stripe_mode character varying(20) DEFAULT 'test'::character varying NOT NULL,
    sepay_mode character varying(20) DEFAULT 'sandbox'::character varying NOT NULL,
    resend_api_key character varying(500) DEFAULT ''::character varying NOT NULL,
    email_from character varying(200) DEFAULT 'DraftRight <noreply@draftright.info>'::character varying NOT NULL,
    client_log_level character varying(10) DEFAULT 'info'::character varying NOT NULL,
    payment_methods_enabled character varying(200) DEFAULT ''::character varying NOT NULL,
    lemonsqueezy_api_key character varying(2000) DEFAULT ''::character varying NOT NULL,
    lemonsqueezy_store_id character varying(50) DEFAULT ''::character varying NOT NULL,
    lemonsqueezy_webhook_secret character varying(100) DEFAULT ''::character varying NOT NULL,
    lemonsqueezy_variant_monthly character varying(50) DEFAULT ''::character varying NOT NULL,
    lemonsqueezy_variant_yearly character varying(50) DEFAULT ''::character varying NOT NULL
);


--
-- Name: bug_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bug_reports (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    source character varying(50) NOT NULL,
    description text NOT NULL,
    screenshot_path character varying(500),
    screenshot_filename character varying(200),
    app_version character varying(50),
    os_info character varying(100),
    user_id uuid,
    user_email character varying(255),
    context jsonb,
    status character varying(20) DEFAULT 'new'::character varying,
    admin_notes text,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    ai_fix_proposal text,
    ai_fix_proposed_at timestamp with time zone,
    kind character varying(20) DEFAULT 'bug'::character varying NOT NULL,
    title character varying(80),
    target_platform character varying(20),
    vote_count integer DEFAULT 0 NOT NULL,
    is_public boolean DEFAULT true NOT NULL,
    display_no bigint NOT NULL
);


--
-- Name: bug_reports_display_no_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.bug_reports_display_no_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: bug_reports_display_no_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.bug_reports_display_no_seq OWNED BY public.bug_reports.display_no;


--
-- Name: email_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    to_email character varying(255) NOT NULL,
    email_type character varying(64) NOT NULL,
    subject character varying(255) NOT NULL,
    status character varying(16) NOT NULL,
    provider_id character varying(255),
    error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: email_suppressions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_suppressions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email character varying(255) NOT NULL,
    reason character varying(255),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: email_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_templates (
    template_key character varying(64) NOT NULL,
    subject character varying(255) NOT NULL,
    html text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: error_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.error_reports (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    platform character varying(20) NOT NULL,
    app_version character varying(50),
    severity character varying(20) DEFAULT 'error'::character varying NOT NULL,
    error_type character varying(200),
    message text,
    stack_trace text,
    context jsonb,
    user_id uuid,
    device_id character varying(100),
    fingerprint character(64) NOT NULL,
    count integer DEFAULT 1 NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    ai_fix_proposal text,
    resolved_by character varying(100),
    resolved_at timestamp with time zone,
    first_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    display_no bigint NOT NULL
);


--
-- Name: error_reports_display_no_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.error_reports_display_no_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: error_reports_display_no_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.error_reports_display_no_seq OWNED BY public.error_reports.display_no;


--
-- Name: extension_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.extension_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token_hash character(64) NOT NULL,
    scopes text[] DEFAULT ARRAY['rewrite'::text] NOT NULL,
    device_id uuid NOT NULL,
    device_name character varying(64) DEFAULT 'mobile'::character varying NOT NULL,
    last_used_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    revoked_at timestamp without time zone
);


--
-- Name: feature_votes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.feature_votes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    feature_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payments (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    amount integer NOT NULL,
    currency character varying(10) DEFAULT 'VND'::character varying NOT NULL,
    method character varying(20) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    provider_ref character varying(255),
    reference_code character varying(50) NOT NULL,
    qr_data text,
    notes text,
    expires_at timestamp without time zone,
    completed_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.plans (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(100) NOT NULL,
    daily_limit integer NOT NULL,
    price_cents integer DEFAULT 0 NOT NULL,
    billing_period public.plans_billing_period_enum DEFAULT 'none'::public.plans_billing_period_enum NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    currency character(3),
    stripe_price_id character varying(255),
    trial_days integer DEFAULT 30 NOT NULL
);


--
-- Name: rewrite_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rewrite_logs (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tone character varying(20) NOT NULL,
    input_text text NOT NULL,
    output_text text NOT NULL,
    model character varying(100) NOT NULL,
    provider_type character varying(20) NOT NULL,
    response_time_ms integer NOT NULL,
    quality character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.subscriptions (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    status public.subscriptions_status_enum NOT NULL,
    store_type public.subscriptions_store_type_enum NOT NULL,
    store_transaction_id character varying(500),
    started_at timestamp without time zone NOT NULL,
    expires_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: usage_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.usage_logs (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    tone character varying(20) NOT NULL,
    input_length integer NOT NULL,
    output_length integer NOT NULL,
    ai_provider_id uuid NOT NULL,
    response_time_ms integer NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255),
    name character varying(255) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    role public.users_role_enum DEFAULT 'user'::public.users_role_enum NOT NULL,
    auth_provider public.users_auth_provider_enum DEFAULT 'local'::public.users_auth_provider_enum NOT NULL,
    google_id character varying(255),
    facebook_id character varying(255),
    tiktok_id character varying(255),
    avatar_url character varying(500),
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    email_verified boolean DEFAULT false NOT NULL,
    email_verification_code character varying(6),
    email_verification_expires timestamp with time zone,
    lemonsqueezy_customer_id character varying(64),
    stripe_customer_id character varying(255),
    apple_id character varying(255),
    password_reset_code character varying(6),
    password_reset_expires timestamp with time zone,
    password_reset_attempts integer DEFAULT 0 NOT NULL
);


--
-- Name: bug_reports display_no; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bug_reports ALTER COLUMN display_no SET DEFAULT nextval('public.bug_reports_display_no_seq'::regclass);


--
-- Name: error_reports display_no; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.error_reports ALTER COLUMN display_no SET DEFAULT nextval('public.error_reports_display_no_seq'::regclass);


--
-- Name: admin_users PK_06744d221bb6145dc61e5dc441d; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_users
    ADD CONSTRAINT "PK_06744d221bb6145dc61e5dc441d" PRIMARY KEY (id);


--
-- Name: payments PK_197ab7af18c93fbb0c9b28b4a59; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT "PK_197ab7af18c93fbb0c9b28b4a59" PRIMARY KEY (id);


--
-- Name: plans PK_3720521a81c7c24fe9b7202ba61; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plans
    ADD CONSTRAINT "PK_3720521a81c7c24fe9b7202ba61" PRIMARY KEY (id);


--
-- Name: usage_logs PK_38ed6efac407c7a3f818d90c279; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usage_logs
    ADD CONSTRAINT "PK_38ed6efac407c7a3f818d90c279" PRIMARY KEY (id);


--
-- Name: app_settings PK_4800b266ba790931744b3e53a74; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app_settings
    ADD CONSTRAINT "PK_4800b266ba790931744b3e53a74" PRIMARY KEY (id);


--
-- Name: rewrite_logs PK_76e231e5a72b1ffbdb58037fbb9; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rewrite_logs
    ADD CONSTRAINT "PK_76e231e5a72b1ffbdb58037fbb9" PRIMARY KEY (id);


--
-- Name: users PK_a3ffb1c0c8416b9fc6f907b7433; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "PK_a3ffb1c0c8416b9fc6f907b7433" PRIMARY KEY (id);


--
-- Name: subscriptions PK_a87248d73155605cf782be9ee5e; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT "PK_a87248d73155605cf782be9ee5e" PRIMARY KEY (id);


--
-- Name: ai_providers PK_de28ebefc0fb425c37b27a4c0a7; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_providers
    ADD CONSTRAINT "PK_de28ebefc0fb425c37b27a4c0a7" PRIMARY KEY (id);


--
-- Name: users UQ_0bd5012aeb82628e07f6a1be53b; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "UQ_0bd5012aeb82628e07f6a1be53b" UNIQUE (google_id);


--
-- Name: payments UQ_10d418900cdf6189f617ac239ca; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT "UQ_10d418900cdf6189f617ac239ca" UNIQUE (reference_code);


--
-- Name: users UQ_723105fc023c3dafb817331fa7e; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "UQ_723105fc023c3dafb817331fa7e" UNIQUE (tiktok_id);


--
-- Name: users UQ_97672ac88f789774dd47f7c8be3; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "UQ_97672ac88f789774dd47f7c8be3" UNIQUE (email);


--
-- Name: admin_users UQ_dcd0c8a4b10af9c986e510b9ecc; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.admin_users
    ADD CONSTRAINT "UQ_dcd0c8a4b10af9c986e510b9ecc" UNIQUE (email);


--
-- Name: users UQ_df199bc6e53abe32d64bbcf2110; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT "UQ_df199bc6e53abe32d64bbcf2110" UNIQUE (facebook_id);


--
-- Name: app_release_policies app_release_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app_release_policies
    ADD CONSTRAINT app_release_policies_pkey PRIMARY KEY (platform);


--
-- Name: app_releases app_releases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.app_releases
    ADD CONSTRAINT app_releases_pkey PRIMARY KEY (platform, channel);


--
-- Name: bug_reports bug_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bug_reports
    ADD CONSTRAINT bug_reports_pkey PRIMARY KEY (id);


--
-- Name: email_logs email_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_logs
    ADD CONSTRAINT email_logs_pkey PRIMARY KEY (id);


--
-- Name: email_templates email_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_templates
    ADD CONSTRAINT email_templates_pkey PRIMARY KEY (template_key);


--
-- Name: error_reports error_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.error_reports
    ADD CONSTRAINT error_reports_pkey PRIMARY KEY (id);


--
-- Name: extension_tokens extension_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.extension_tokens
    ADD CONSTRAINT extension_tokens_pkey PRIMARY KEY (id);


--
-- Name: feature_votes feature_votes_feature_id_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feature_votes
    ADD CONSTRAINT feature_votes_feature_id_user_id_key UNIQUE (feature_id, user_id);


--
-- Name: feature_votes feature_votes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feature_votes
    ADD CONSTRAINT feature_votes_pkey PRIMARY KEY (id);


--
-- Name: plans plans_name_currency_period_uniq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.plans
    ADD CONSTRAINT plans_name_currency_period_uniq UNIQUE (name, currency, billing_period);


--
-- Name: IDX_00e6e0715710102f93baf55f12; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX "IDX_00e6e0715710102f93baf55f12" ON public.usage_logs USING btree (user_id, created_at);


--
-- Name: IDX_30f84eb83d87c4c339b9163bef; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX "IDX_30f84eb83d87c4c339b9163bef" ON public.rewrite_logs USING btree (tone, created_at);


--
-- Name: IDX_58620d2791557a4d0f44e87744; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX "IDX_58620d2791557a4d0f44e87744" ON public.payments USING btree (status, created_at);


--
-- Name: IDX_8eac494f9f6215a0e65c0135bb; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX "IDX_8eac494f9f6215a0e65c0135bb" ON public.payments USING btree (user_id, created_at);


--
-- Name: bug_reports_display_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX bug_reports_display_no_idx ON public.bug_reports USING btree (display_no);


--
-- Name: bug_reports_kind_public_votes_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX bug_reports_kind_public_votes_idx ON public.bug_reports USING btree (kind, is_public, vote_count);


--
-- Name: bug_reports_source_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX bug_reports_source_idx ON public.bug_reports USING btree (source);


--
-- Name: bug_reports_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX bug_reports_status_idx ON public.bug_reports USING btree (status, created_at DESC);


--
-- Name: error_reports_display_no_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX error_reports_display_no_idx ON public.error_reports USING btree (display_no);


--
-- Name: feature_votes_feature_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX feature_votes_feature_idx ON public.feature_votes USING btree (feature_id);


--
-- Name: idx_email_logs_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_email_logs_created ON public.email_logs USING btree (created_at DESC);


--
-- Name: idx_email_logs_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_email_logs_status ON public.email_logs USING btree (status);


--
-- Name: idx_email_logs_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_email_logs_type ON public.email_logs USING btree (email_type);


--
-- Name: idx_ext_tokens_hash_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_ext_tokens_hash_active ON public.extension_tokens USING btree (token_hash) WHERE (revoked_at IS NULL);


--
-- Name: idx_ext_tokens_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ext_tokens_user ON public.extension_tokens USING btree (user_id);


--
-- Name: idx_ext_tokens_user_device_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_ext_tokens_user_device_active ON public.extension_tokens USING btree (user_id, device_id) WHERE (revoked_at IS NULL);


--
-- Name: ix_errors_fingerprint; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_errors_fingerprint ON public.error_reports USING btree (fingerprint);


--
-- Name: ix_errors_last_seen; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_errors_last_seen ON public.error_reports USING btree (last_seen_at DESC);


--
-- Name: ix_errors_platform_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_errors_platform_status ON public.error_reports USING btree (platform, status);


--
-- Name: subscriptions_store_txn_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX subscriptions_store_txn_idx ON public.subscriptions USING btree (store_transaction_id) WHERE (store_transaction_id IS NOT NULL);


--
-- Name: users_stripe_customer_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX users_stripe_customer_id_idx ON public.users USING btree (stripe_customer_id) WHERE (stripe_customer_id IS NOT NULL);


--
-- Name: usage_logs FK_0ae4be269441a06bd47c97c6928; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usage_logs
    ADD CONSTRAINT "FK_0ae4be269441a06bd47c97c6928" FOREIGN KEY (ai_provider_id) REFERENCES public.ai_providers(id);


--
-- Name: payments FK_427785468fb7d2733f59e7d7d39; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT "FK_427785468fb7d2733f59e7d7d39" FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: usage_logs FK_7b39630636b5b0339d718dd8a11; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.usage_logs
    ADD CONSTRAINT "FK_7b39630636b5b0339d718dd8a11" FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: subscriptions FK_d0a95ef8a28188364c546eb65c1; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT "FK_d0a95ef8a28188364c546eb65c1" FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: subscriptions FK_e45fca5d912c3a2fab512ac25dc; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.subscriptions
    ADD CONSTRAINT "FK_e45fca5d912c3a2fab512ac25dc" FOREIGN KEY (plan_id) REFERENCES public.plans(id);


--
-- Name: payments FK_f9b6a4c3196864cdd91b1a440ee; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT "FK_f9b6a4c3196864cdd91b1a440ee" FOREIGN KEY (plan_id) REFERENCES public.plans(id);


--
-- Name: bug_reports bug_reports_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bug_reports
    ADD CONSTRAINT bug_reports_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: error_reports error_reports_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.error_reports
    ADD CONSTRAINT error_reports_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: extension_tokens extension_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.extension_tokens
    ADD CONSTRAINT extension_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: feature_votes feature_votes_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feature_votes
    ADD CONSTRAINT feature_votes_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.bug_reports(id) ON DELETE CASCADE;


--
-- Name: feature_votes feature_votes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.feature_votes
    ADD CONSTRAINT feature_votes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict Iw8RjWS8hHTbdhKdf8qIdIA6VincHu4mwbki5Enci8WJwJNminJ57HRkYSWPRaN

