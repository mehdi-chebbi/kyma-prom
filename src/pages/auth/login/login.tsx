import { useState } from 'react';
import "./login.css"
import { useNavigate } from 'react-router-dom';
import { login } from '../../../services/userService';
import hideIcon from '../../../assets/eye-password-hide.svg'
import showIcon from '../../../assets/eye-password-show.svg'

const Login = () => {
  const navigate = useNavigate();

  const [uid, setUid] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const result = await login({ uid, password });

      // Store the token
      localStorage.setItem('authToken', result.login.token);

      // Store user info
      localStorage.setItem('user', JSON.stringify(result.login.user));

      navigate("/dashboard");
    } catch (err: any) {
      setError(err.message || 'Login failed. Please check your credentials.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center ">
      <div className="bg-white p-8 rounded-lg shadow-lg w-full max-w-sm card">
        <h2 className="text-2xl font-bold mb-6 text-center text-black">Login</h2>

        {error && (
          <div className="mb-4 p-3 bg-red-100 border border-red-400 text-red-700 rounded">
            {error}
          </div>
        )}

        <form onSubmit={handleLogin} className="space-y-4">
          <div>
            <label htmlFor='username-input' className="block text-gray-700">Username (UID)</label>
            <input
              id="username-input"
              type="text"
              value={uid}
              onChange={(e) => setUid(e.target.value)}
              className="w-full px-4 py-2 border focus:outline-none focus:ring-2 focus:ring-blue-400 login-input"
              placeholder="Enter your username (e.g., john.doe)"
              required
              disabled={loading}
            />
          </div>
          <div>
            <label htmlFor='password-input' className="block text-gray-700">Password</label>
              <div className="relative">
                <input
            id="password-input"
            type={showPassword ? "text" : "password"}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full px-4 py-2 pr-10 border focus:outline-none focus:ring-2 focus:ring-blue-400 login-input"
            placeholder="Enter your password"
            required
            disabled={loading}
          />


<button
  type="button"
  onClick={() => setShowPassword(prev => !prev)}
  className="password-toggle-btn"
  aria-label={showPassword ? "Hide password" : "Show password"}
  aria-pressed={showPassword}
  aria-controls="password-input"
>
  <img
    src={showPassword ? hideIcon : showIcon}
    alt=""
    aria-hidden="true"
    className="password-icon"
  />
</button>



            </div>
          </div>
          <button
            type="submit"
            className="w-full bg-blue-500 text-white py-2 rounded-lg hover:bg-blue-600 transition-colors submit-button disabled:opacity-50 disabled:cursor-not-allowed"
            disabled={loading}
          >
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>
        </div>
      </div>
  );
};

export default Login;
