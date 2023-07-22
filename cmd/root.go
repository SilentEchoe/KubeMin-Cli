package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var cfgFile string

// 构建根 command 命令。前面我们介绍它还可以有子命令，这个command里没有构建子命令
var rootCmd = &cobra.Command{
	Use:   "KubeMin-CLI",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
}

// 执行 rootCmd 命令并检测错误
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	// 加载运行初始化配置
	cobra.OnInitialize(initConfig)
	// rootCmd，命令行下读取配置文件，持久化的 flag，全局的配置文件
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.firstappname.yaml)")
	// local flag，本地化的配置
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// 初始化配置的一些设置
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile) // viper 设置配置文件
	} else { // 上面没有指定配置文件，下面就读取 home 下的 .firstappname.yaml文件
		// 配置文件参数设置
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".firstappname")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil { // 读取配置文件
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
