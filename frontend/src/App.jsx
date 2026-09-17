import React, { useEffect, useState } from "react";
import { Link, Navigate, Route, Routes, useNavigate, useParams } from "react-router-dom";
import { request, websocketURL } from "./api";

function Layout({ children }) {
  const token = localStorage.getItem("token");
  const navigate = useNavigate();

  function logout() {
    localStorage.removeItem("token");
    localStorage.removeItem("user");
    navigate("/login");
  }

  return (
    <div className="app">
      <header>
        <Link to="/" className="brand">LivePoll</Link>
        <nav>
          {token ? (
            <>
              <Link to="/create">Create Poll</Link>
              <button className="link-button" onClick={logout}>Logout</button>
            </>
          ) : (
            <>
              <Link to="/login">Login</Link>
              <Link to="/signup" className="nav-cta">Sign up</Link>
            </>
          )}
        </nav>
      </header>
      <main>{children}</main>
    </div>
  );
}

function Home() {
  return (
    <section className="hero">
      <div className="hero-card">
        <p className="eyebrow">REAL-TIME POLLING</p>
        <h1>Ask a question.<br />See answers live.</h1>
        <p className="subtitle">
          Create a poll, share one simple link, and watch the results update instantly as people vote.
        </p>
        <div className="actions">
          <Link className="primary" to={localStorage.getItem("token") ? "/create" : "/signup"}>
            Create a poll
          </Link>
          <Link className="secondary" to="/login">I already have an account</Link>
        </div>
      </div>
    </section>
  );
}

function Auth({ mode }) {
  const navigate = useNavigate();
  const [form, setForm] = useState({ name: "", email: "", password: "" });
  const [error, setError] = useState("");

  async function submit(e) {
    e.preventDefault();
    setError("");
    try {
      const data = await request(`/api/auth/${mode}`, {
        method: "POST",
        body: JSON.stringify(form),
      });
      localStorage.setItem("token", data.token);
      localStorage.setItem("user", JSON.stringify(data.user));
      navigate("/create");
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <section className="form-page">
      <form className="card form-card" onSubmit={submit}>
        <p className="eyebrow">{mode === "login" ? "WELCOME BACK" : "GET STARTED"}</p>
        <h2>{mode === "login" ? "Log in" : "Create your account"}</h2>
        {mode === "signup" && (
          <input placeholder="Name" value={form.name}
            onChange={e => setForm({...form, name: e.target.value})} />
        )}
        <input type="email" placeholder="Email" value={form.email}
          onChange={e => setForm({...form, email: e.target.value})} required />
        <input type="password" placeholder="Password (6+ characters)" value={form.password}
          onChange={e => setForm({...form, password: e.target.value})} required minLength="6" />
        {error && <p className="error">{error}</p>}
        <button className="primary full" type="submit">
          {mode === "login" ? "Log in" : "Sign up"}
        </button>
        <p className="muted">
          {mode === "login" ? "New here? " : "Already have an account? "}
          <Link to={mode === "login" ? "/signup" : "/login"}>
            {mode === "login" ? "Sign up" : "Log in"}
          </Link>
        </p>
      </form>
    </section>
  );
}

function CreatePoll() {
  const navigate = useNavigate();
  const [title, setTitle] = useState("");
  const [options, setOptions] = useState(["", ""]);
  const [error, setError] = useState("");

  if (!localStorage.getItem("token")) return <Navigate to="/login" />;

  function updateOption(index, value) {
    setOptions(options.map((x, i) => i === index ? value : x));
  }

  async function submit(e) {
    e.preventDefault();
    setError("");
    try {
      const poll = await request("/api/polls", {
        method: "POST",
        body: JSON.stringify({ title, options }),
      });
      navigate(`/poll/${poll.id}`);
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <section className="form-page">
      <form className="card create-card" onSubmit={submit}>
        <p className="eyebrow">NEW POLL</p>
        <h2>Create your question</h2>
        <input placeholder="e.g. Which language do you prefer?"
          value={title} onChange={e => setTitle(e.target.value)} maxLength="200" required />
        <div className="options">
          {options.map((option, i) => (
            <input key={i} placeholder={`Option ${i + 1}`} value={option}
              onChange={e => updateOption(i, e.target.value)} maxLength="100" required />
          ))}
        </div>
        {options.length < 10 && (
          <button type="button" className="secondary" onClick={() => setOptions([...options, ""])}>
            + Add option
          </button>
        )}
        {options.length > 2 && (
          <button type="button" className="text-danger"
            onClick={() => setOptions(options.slice(0, -1))}>Remove last option</button>
        )}
        {error && <p className="error">{error}</p>}
        <button className="primary full" type="submit">Create poll</button>
      </form>
    </section>
  );
}

function PollPage() {
  const { id } = useParams();
  const [poll, setPoll] = useState(null);
  const [counts, setCounts] = useState({});
  const [selected, setSelected] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  async function load() {
    try {
      const [p, r] = await Promise.all([
        request(`/api/polls/${id}`),
        request(`/api/polls/${id}/results`)
      ]);
      setPoll(p);
      setCounts(r.counts || {});
    } catch (err) {
      setError(err.message);
    }
  }

  useEffect(() => {
    load();
    const ws = new WebSocket(websocketURL(id));

    ws.onmessage = event => {
      try {
        const data = JSON.parse(event.data);
        if (data.optionId) {
          setCounts(prev => ({ ...prev, [data.optionId]: data.count }));
        }
      } catch {}
    };

    return () => ws.close();
  }, [id]);

  async function vote() {
    if (!selected) return;
    setMessage("");
    setError("");
    let voterKey = localStorage.getItem("voterKey");
    if (!voterKey) {
      voterKey = crypto.randomUUID();
      localStorage.setItem("voterKey", voterKey);
    }

    try {
      await request(`/api/polls/${id}/vote`, {
        method: "POST",
        body: JSON.stringify({ optionId: selected, voterKey }),
      });
      setMessage("Vote recorded. Results are live.");
      setSelected("");
    } catch (err) {
      setError(err.message);
    }
  }

  if (error && !poll) return <section className="center"><p className="error">{error}</p></section>;
  if (!poll) return <section className="center">Loading poll...</section>;

  const total = Object.values(counts).reduce((a, b) => a + Number(b), 0);

  return (
    <section className="poll-page">
      <div className="poll-header">
        <p className="eyebrow">LIVE POLL</p>
        <h1>{poll.title}</h1>
        <p className="muted">{total} vote{total === 1 ? "" : "s"} so far</p>
      </div>

      <div className="poll-grid">
        <div className="card vote-card">
          <h3>Cast your vote</h3>
          {poll.options.map(option => (
            <label className={`option ${selected === option.id ? "selected" : ""}`} key={option.id}>
              <input type="radio" name="option" value={option.id}
                checked={selected === option.id}
                onChange={() => setSelected(option.id)} />
              <span>{option.text}</span>
            </label>
          ))}
          <button className="primary full" onClick={vote} disabled={!selected}>Vote</button>
          {message && <p className="success">{message}</p>}
          {error && <p className="error">{error}</p>}
        </div>

        <div className="card results-card">
          <div className="results-title">
            <h3>Live results</h3>
            <span className="live-dot">● LIVE</span>
          </div>
          {poll.options.map(option => {
            const value = Number(counts[option.id] || 0);
            const percent = total ? Math.round(value / total * 100) : 0;
            return (
              <div className="result" key={option.id}>
                <div className="result-label">
                  <span>{option.text}</span>
                  <strong>{value} · {percent}%</strong>
                </div>
                <div className="bar"><div style={{width: `${percent}%`}} /></div>
              </div>
            );
          })}
          <p className="muted realtime-note">Open this poll in another tab and vote to test live updates.</p>
        </div>
      </div>
    </section>
  );
}

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/login" element={<Auth mode="login" />} />
        <Route path="/signup" element={<Auth mode="signup" />} />
        <Route path="/create" element={<CreatePoll />} />
        <Route path="/poll/:id" element={<PollPage />} />
      </Routes>
    </Layout>
  );
}
