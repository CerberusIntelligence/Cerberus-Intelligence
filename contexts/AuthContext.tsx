import React, { createContext, useContext, useState, useEffect } from 'react';
import { User, UserAccess, AuthContextType } from '../types';
import { supabase, authHelpers, accessHelpers } from '../services/supabaseClient';

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [userAccess, setUserAccess] = useState<UserAccess | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Check if user has active (non-expired) access
  const hasActiveAccess = accessHelpers.hasActiveAccess(userAccess);

  // Initialize auth state and listen for changes
  useEffect(() => {
    // Check current session on mount
    checkUser();

    // Listen for auth state changes
    const { data: { subscription } } = supabase.auth.onAuthStateChange(
      async (event, session) => {
        if (session?.user) {
          await loadUserData(session.user.id);
        } else {
          setUser(null);
          setUserAccess(null);
        }
        setIsLoading(false);
      }
    );

    return () => {
      subscription.unsubscribe();
    };
  }, []);

  // Check current user and load their data
  async function checkUser() {
    try {
      const currentUser = await authHelpers.getCurrentUser();
      if (currentUser) {
        await loadUserData(currentUser.id);
      }
    } catch (error) {
      console.error('Error checking user:', error);
    } finally {
      setIsLoading(false);
    }
  }

  // Load user data including access information
  async function loadUserData(userId: string) {
    try {
      const { data: { user: authUser }, error: userError } = await supabase.auth.getUser();

      if (userError) throw userError;

      if (authUser) {
        setUser({
          id: authUser.id,
          email: authUser.email || '',
          created_at: authUser.created_at || new Date().toISOString(),
        });

        // Fetch user access data
        const access = await accessHelpers.getUserAccess(userId);
        setUserAccess(access);
      }
    } catch (error) {
      console.error('Error loading user data:', error);
    }
  }

  // Sign up a new user
  async function signUp(email: string, password: string) {
    try {
      setIsLoading(true);
      const { user: newUser } = await authHelpers.signUp(email, password);

      if (newUser) {
        await loadUserData(newUser.id);
      }
    } catch (error) {
      console.error('Sign up error:', error);
      throw error;
    } finally {
      setIsLoading(false);
    }
  }

  // Sign in existing user
  async function signIn(email: string, password: string) {
    try {
      setIsLoading(true);
      const { user: signedInUser } = await authHelpers.signIn(email, password);

      if (signedInUser) {
        await loadUserData(signedInUser.id);
      }
    } catch (error) {
      console.error('Sign in error:', error);
      throw error;
    } finally {
      setIsLoading(false);
    }
  }

  // Sign out current user
  async function signOut() {
    try {
      setIsLoading(true);
      await authHelpers.signOut();
      setUser(null);
      setUserAccess(null);
    } catch (error) {
      console.error('Sign out error:', error);
      throw error;
    } finally {
      setIsLoading(false);
    }
  }

  // Check if user has valid access (not expired)
  async function checkAccess(): Promise<boolean> {
    if (!user) return false;

    // Refresh user access data
    const access = await accessHelpers.getUserAccess(user.id);
    setUserAccess(access);

    return accessHelpers.hasActiveAccess(access);
  }

  const value: AuthContextType = {
    user,
    userAccess,
    isLoading,
    signIn,
    signUp,
    signOut,
    checkAccess,
    hasActiveAccess,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// Custom hook to use auth context
export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
